// Package interpreter - stdlib.go intercepts standard library calls that would
// otherwise return opaque zero values (#42, #43).
//
// When the interpreter encounters a call to strings.*, strconv.*, or fmt.*,
// the callee has no interpretable SSA body (Blocks == nil). Without intercepts,
// execCall returns Value{} for these, causing downstream branches to always
// take the false/zero path and missing reachable violations.
//
// For concrete arguments the real Go stdlib function is called directly.
// For non-concrete arguments (Value{Raw: nil}) a pessimistic non-zero return is
// used: bool predicates return true, string-returning functions return a
// non-empty sentinel, numeric functions return a non-zero sentinel.
package interpreter

import (
	"encoding/asn1"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"math/bits"
	"math/cmplx"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/scttfrdmn/giri/pkg/shadow"
	"golang.org/x/tools/go/ssa"
)

// stdlibOpaque is a shared sentinel Value used by stdlib intercept handlers to
// represent an opaque non-nil result (e.g. an un-modeled struct or interface
// pointer). Using a package-level var avoids 100+ identical local declarations.
var stdlibOpaque = Value{Raw: struct{}{}}

// execStdlibCall intercepts standard library function calls in packages
// "strings", "strconv", "fmt", "time", "os", "math/rand", "bytes",
// "errors", "sort", "sync/atomic", "io", "bufio", "log",
// "encoding/hex", "encoding/base64", "encoding/xml", "encoding/csv",
// "crypto/rand", "crypto/md5", "crypto/sha1", "crypto/sha256",
// "path/filepath", "path", "net", "net/url", "text/template", "html/template",
// "reflect", "flag", "runtime", "os/exec", "compress/gzip", "compress/zlib",
// "net/http", "os/signal", "encoding/binary", "hash/crc32", "hash/fnv",
// "hash/adler32", "container/list", "container/heap", "container/ring",
// "math/big", "crypto/tls", "database/sql", "testing",
// "io/fs", "embed", "archive/zip", "archive/tar",
// "mime", "mime/multipart", "crypto/cipher", "crypto/aes", "crypto/hmac",
// "slices", "maps", "cmp", "log/slog", "iter",
// "math/bits", "math/cmplx", "html", "unicode/utf16", "os/user",
// "runtime/debug", "net/netip",
// "math/rand/v2", "encoding/pem", "encoding/asn1",
// "crypto/rsa", "crypto/ecdsa", "crypto/ed25519", "crypto/ecdh",
// "crypto/x509", "runtime/pprof", "runtime/trace",
// "golang.org/x/sync/errgroup", "golang.org/x/sync/singleflight",
// "encoding/gob", "encoding/base32",
// "image", "image/color", "image/draw", "image/png", "image/jpeg", "image/gif",
// "expvar", "text/tabwriter", "text/scanner",
// "net/smtp", "net/mail", "net/textproto",
// "go/token", "go/ast", "go/parser", "go/format",
// "syscall", "testing/iotest", "testing/fstest",
// "net/http/httptest", "net/http/httputil", "net/rpc",
// "debug/pprof", "net/http/pprof", "plugin",
// "golang.org/x/sync/semaphore",
// "io/ioutil", "compress/bzip2", "compress/flate", "compress/lzw",
// "go/types", "go/importer", "go/build", "go/doc", "net/http/cookiejar",
// "crypto/subtle", "hash/maphash", "regexp/syntax", "unique",
// "go/printer", "go/constant", "go/scanner", "go/version",
// "debug/buildinfo", "debug/dwarf", "debug/elf", "debug/macho", "debug/pe",
// "testing/quick", "mime/quotedprintable", "net/http/httptrace", "net/rpc/jsonrpc",
// "go/build/constraint", "go/doc/comment", "text/template/parse",
// "debug/gosym", "debug/plan9obj", "runtime/metrics", "runtime/coverage",
// "net/http/cgi", "net/http/fcgi", "encoding/ascii85", "index/suffixarray", "log/syslog",
// "crypto/dsa", "crypto/elliptic", "hash/crc64",
// "golang.org/x/crypto/bcrypt", "golang.org/x/net/http2".
// strings.NewReader and bytes.NewReader/NewBuffer/NewBufferString are handled
// within the existing "strings" and "bytes" cases.
// Returns (result, true) when intercepted, (Value{}, false) otherwise.
//
// gid and site are required by handlers that invoke user callbacks
// (e.g. sort.Slice calls the less function via execFunction).
func (interp *Interpreter) execStdlibCall(gid int64, site, pkgPath, name string, args []Value) (Value, bool) {
	// User-registered intercepts (#113) take priority over built-in handlers,
	// allowing overrides of both external libraries and stdlib functions.
	if fns, ok := interp.config.Intercepts[pkgPath]; ok {
		if fn, ok := fns[name]; ok {
			return fn(args)
		}
	}

	// Skip dependency package init() calls (#146). The main package's own
	// init is invoked directly in Run() before main(). Any call to a function
	// literally named "init" that reaches execCall is a dependency package's
	// synthesized init (e.g. runtime.init, fmt.init), which initializes runtime
	// internals that the interpreter cannot model. User-defined init functions
	// are renamed init$1, init$2, etc. in SSA and are NOT filtered here.
	if name == "init" {
		return Value{}, true
	}

	switch pkgPath {
	case "strings":
		return interp.handleStringsCall(name, args)
	case "strconv":
		return interp.handleStrconvCall(name, args)
	case "fmt":
		return interp.handleFmtCall(name, args)
	case "time":
		return interp.handleTimeCall(name, args)
	case "os":
		return interp.handleOSCall(name, args)
	case "math/rand":
		return interp.handleMathRandCall(name, args)
	case "bytes":
		return interp.handleBytesCall(name, args)
	case "errors":
		return interp.handleErrorsCall(name, args)
	case "sort":
		return interp.handleSortCall(gid, name, args, site)
	case "encoding/json":
		return interp.handleJSONCall(name, args)
	case "regexp":
		return interp.handleRegexpCall(gid, name, args, site)
	case "math":
		return interp.handleMathCall(name, args)
	case "unicode/utf8":
		return interp.handleUTF8Call(name, args)
	case "unicode":
		return interp.handleUnicodeCall(name, args)
	case "context":
		return interp.handleContextCall(gid, site, name, args)
	case "sync/atomic":
		return interp.handleAtomicCall(name, args)
	case "io":
		return interp.handleIOCall(name, args)
	case "bufio":
		return interp.handleBufioCall(name, args)
	case "log":
		return interp.handleLogCall(gid, name, args)
	case "encoding/hex":
		return interp.handleHexCall(name, args)
	case "encoding/base64":
		return interp.handleBase64Call(name, args)
	case "crypto/rand":
		return interp.handleCryptoRandCall(name, args)
	case "crypto/md5", "crypto/sha1", "crypto/sha256", "crypto/sha512":
		return interp.handleHashCall(pkgPath, name, args)
	case "path/filepath":
		return interp.handleFilepathCall(name, args)
	case "path":
		return interp.handlePathCall(name, args)
	case "net":
		return interp.handleNetCall(name, args)
	case "text/template", "html/template":
		return interp.handleTemplateCall(name, args)
	case "encoding/xml":
		return interp.handleXMLCall(name, args)
	case "encoding/csv":
		return interp.handleCSVCall(name, args)
	case "reflect":
		return interp.handleReflectCall(name, args)
	case "flag":
		return interp.handleFlagCall(name, args)
	case "runtime":
		return interp.handleRuntimeCall(name, args)
	case "net/url":
		return interp.handleNetURLCall(name, args)
	case "os/exec":
		return interp.handleExecCall(name, args)
	case "compress/gzip":
		return interp.handleGzipCall(name, args)
	case "compress/zlib":
		return interp.handleZlibCall(name, args)
	case "net/http":
		return interp.handleHTTPCall(name, args)
	case "os/signal":
		return interp.handleSignalCall(gid, name, args)
	case "encoding/binary":
		return interp.handleBinaryCall(name, args)
	case "hash/crc32", "hash/fnv", "hash/adler32":
		return interp.handleHashExtCall(pkgPath, name, args)
	case "container/list", "container/heap", "container/ring":
		return interp.handleContainerCall(gid, pkgPath, name, args)
	case "math/big":
		return interp.handleMathBigCall(name, args)
	case "crypto/tls":
		return interp.handleTLSCall(name, args)
	case "database/sql":
		return interp.handleSQLCall(name, args)
	case "testing":
		return interp.handleTestingCall(gid, name, args)
	case "io/fs", "embed":
		return interp.handleFsCall(pkgPath, name, args)
	case "archive/zip", "archive/tar":
		return interp.handleArchiveCall(pkgPath, name, args)
	case "mime", "mime/multipart":
		return interp.handleMimeCall(pkgPath, name, args)
	case "crypto/cipher", "crypto/aes", "crypto/hmac":
		return interp.handleSymCryptoCall(pkgPath, name, args)
	case "golang.org/x/tools/go/packages":
		return interp.handleGoPackagesCall(name, args)
	case "slices":
		return interp.handleSlicesCall(gid, name, args, site)
	case "maps":
		return interp.handleMapsCall(gid, name, args, site)
	case "cmp":
		return interp.handleCmpCall(name, args)
	case "log/slog":
		return interp.handleSlogCall(name, args)
	case "iter":
		return interp.handleIterCall(name, args)
	case "math/bits":
		return interp.handleMathBitsCall(name, args)
	case "math/cmplx":
		return interp.handleMathCmplxCall(name, args)
	case "html":
		return interp.handleHTMLCall(name, args)
	case "unicode/utf16":
		return interp.handleUTF16Call(name, args)
	case "os/user":
		return interp.handleOSUserCall(name, args)
	case "runtime/debug":
		return interp.handleRuntimeDebugCall(name, args)
	case "net/netip":
		return interp.handleNetNetipCall(name, args)
	case "math/rand/v2":
		return interp.handleMathRandV2Call(name, args)
	case "encoding/pem":
		return interp.handleEncodingPEMCall(name, args)
	case "encoding/asn1":
		return interp.handleEncodingASN1Call(name, args)
	case "crypto/rsa":
		return interp.handleCryptoRSACall(name, args)
	case "crypto/ecdsa":
		return interp.handleCryptoECDSACall(name, args)
	case "crypto/ed25519":
		return interp.handleCryptoEd25519Call(name, args)
	case "crypto/ecdh":
		return interp.handleCryptoECDHCall(name, args)
	case "crypto/x509":
		return interp.handleCryptoX509Call(name, args)
	case "runtime/pprof":
		return interp.handleRuntimePprofCall(name, args)
	case "runtime/trace":
		return interp.handleRuntimeTraceCall(name, args)
	case "golang.org/x/sync/errgroup":
		return interp.handleErrgroupCall(gid, name, args, site)
	case "golang.org/x/sync/singleflight":
		return interp.handleSingleflightCall(gid, name, args, site)
	case "encoding/gob":
		return interp.handleEncodingGobCall(name, args)
	case "encoding/base32":
		return interp.handleEncodingBase32Call(name, args)
	case "image", "image/color", "image/draw":
		return interp.handleImageCall(pkgPath, name, args)
	case "image/png", "image/jpeg", "image/gif":
		return interp.handleImageCodecCall(pkgPath, name, args)
	case "expvar":
		return interp.handleExpvarCall(name, args)
	case "text/tabwriter":
		return interp.handleTabwriterCall(name, args)
	case "text/scanner":
		return interp.handleTextScannerCall(name, args)
	case "net/smtp":
		return interp.handleNetSMTPCall(name, args)
	case "net/mail":
		return interp.handleNetMailCall(name, args)
	case "net/textproto":
		return interp.handleNetTextprotoCall(name, args)
	case "go/token":
		return interp.handleGoTokenCall(gid, name, args)
	case "go/ast":
		return interp.handleGoASTCall(gid, site, name, args)
	case "go/parser":
		return interp.handleGoParserCall(name, args)
	case "go/format":
		return interp.handleGoFormatCall(name, args)
	case "syscall":
		return interp.handleSyscallCall(name, args)
	case "testing/iotest":
		return interp.handleTestingIotestCall(name, args)
	case "testing/fstest":
		return interp.handleTestingFstestCall(name, args)
	case "net/http/httptest":
		return interp.handleHTTPTestCall(name, args)
	case "net/http/httputil":
		return interp.handleHTTPUtilCall(name, args)
	case "net/rpc":
		return interp.handleNetRPCCall(name, args)
	case "debug/pprof":
		return interp.handleDebugPprofCall(name, args)
	case "net/http/pprof":
		return interp.handleNetHTTPPprofCall(name, args)
	case "plugin":
		return interp.handlePluginCall(name, args)
	case "golang.org/x/sync/semaphore":
		return interp.handleSemaphoreCall(name, args)
	// v0.57.0 additions (#169-172).
	case "io/ioutil":
		return interp.handleIOIoutilCall(name, args)
	case "compress/bzip2":
		return interp.handleCompressBzip2Call(name, args)
	case "compress/flate":
		return interp.handleCompressFlateCall(name, args)
	case "compress/lzw":
		return interp.handleCompressLZWCall(name, args)
	case "go/types":
		return interp.handleGoTypesCall(gid, site, name, args)
	case "go/importer":
		return interp.handleGoImporterCall(name, args)
	case "go/build":
		return interp.handleGoBuildCall(name, args)
	case "go/doc":
		return interp.handleGoDocCall(name, args)
	case "net/http/cookiejar":
		return interp.handleCookiejarCall(name, args)
	// v0.58.0 additions (#173-176).
	case "crypto/subtle":
		return interp.handleCryptoSubtleCall(name, args)
	case "hash/maphash":
		return interp.handleMapHashCall(name, args)
	case "regexp/syntax":
		return interp.handleRegexpSyntaxCall(name, args)
	case "unique":
		return interp.handleUniqueCall(name, args)
	case "go/printer":
		return interp.handleGoPrinterCall(name, args)
	case "go/constant":
		return interp.handleGoConstantCall(name, args)
	case "go/scanner":
		return interp.handleGoScannerCall(name, args)
	case "go/version":
		return interp.handleGoVersionCall(name, args)
	case "debug/buildinfo":
		return interp.handleDebugBuildinfoCall(name, args)
	case "debug/dwarf":
		return interp.handleDebugDWARFCall(name, args)
	case "debug/elf":
		return interp.handleDebugELFCall(name, args)
	case "debug/macho":
		return interp.handleDebugMachoCall(name, args)
	case "debug/pe":
		return interp.handleDebugPECall(name, args)
	case "testing/quick":
		return interp.handleTestingQuickCall(gid, site, name, args)
	case "mime/quotedprintable":
		return interp.handleQuotedPrintableCall(name, args)
	case "net/http/httptrace":
		return interp.handleHTTPTraceCall(name, args)
	case "net/rpc/jsonrpc":
		return interp.handleJSONRPCCall(name, args)
	// v0.59.0 additions (#177-180).
	case "go/build/constraint":
		return interp.handleGoBuildConstraintCall(name, args)
	case "go/doc/comment":
		return interp.handleGoDocCommentCall(name, args)
	case "text/template/parse":
		return interp.handleTemplateParseCall(name, args)
	case "debug/gosym":
		return interp.handleDebugGosymCall(name, args)
	case "debug/plan9obj":
		return interp.handleDebugPlan9Call(name, args)
	case "runtime/metrics":
		return interp.handleRuntimeMetricsCall(name, args)
	case "runtime/coverage":
		return interp.handleRuntimeCoverageCall(name, args)
	case "net/http/cgi":
		return interp.handleHTTPCGICall(name, args)
	case "net/http/fcgi":
		return interp.handleHTTPFCGICall(name, args)
	case "encoding/ascii85":
		return interp.handleASCII85Call(name, args)
	case "index/suffixarray":
		return interp.handleSuffixArrayCall(name, args)
	case "log/syslog":
		return interp.handleSyslogCall(name, args)
	case "crypto/dsa":
		return interp.handleCryptoDSACall(name, args)
	case "crypto/elliptic":
		return interp.handleCryptoEllipticCall(name, args)
	case "hash/crc64":
		return interp.handleHashCRC64Call(name, args)
	case "golang.org/x/crypto/bcrypt":
		return interp.handleBcryptCall(name, args)
	case "golang.org/x/net/http2":
		return interp.handleNetHTTP2Call(name, args)
	// v0.60.0 additions (#181-184).
	case "crypto/des":
		return interp.handleCryptoDESCall(name, args)
	case "crypto/rc4":
		return interp.handleCryptoRC4Call(name, args)
	case "crypto/pbkdf2":
		return interp.handleCryptoPBKDF2Call(name, args)
	case "crypto/hkdf":
		return interp.handleCryptoHKDFCall(name, args)
	case "crypto/sha3":
		return interp.handleCryptoSHA3Call(name, args)
	case "crypto/hpke":
		return interp.handleCryptoHPKECall(name, args)
	case "crypto/mlkem":
		return interp.handleCryptoMLKEMCall(name, args)
	case "crypto/fips140":
		return interp.handleCryptoFIPS140Call(name, args)
	case "database/sql/driver":
		return interp.handleSQLDriverCall(name, args)
	case "crypto/x509/pkix":
		return interp.handleX509PKIXCall(name, args)
	case "image/color/palette":
		return interp.handleColorPaletteCall(name, args)
	case "time/tzdata":
		return interp.handleTZDataCall(name, args)
	case "structs":
		return interp.handleStructsCall(name, args)
	case "weak":
		return interp.handleWeakCall(name, args)
	case "testing/slogtest":
		return interp.handleSlogTestCall(name, args)
	case "testing/synctest":
		return interp.handleSyncTestCall(name, args)
	case "golang.org/x/sys/unix":
		return interp.handleSysUnixCall(name, args)
	case "golang.org/x/net/html":
		return interp.handleNetHTMLCall(gid, site, name, args)
	case "golang.org/x/net/html/charset":
		return interp.handleNetHTMLCharsetCall(name, args)
	case "golang.org/x/net/publicsuffix":
		return interp.handleNetPublicSuffixCall(name, args)
	case "golang.org/x/net/idna":
		return interp.handleNetIDNACall(name, args)
	case "golang.org/x/net/proxy":
		return interp.handleNetProxyCall(gid, site, name, args)
	case "golang.org/x/net/netutil":
		return interp.handleNetNetUtilCall(name, args)
	case "golang.org/x/net/http/httpguts":
		return interp.handleHTTPGutsCall(name, args)
	case "golang.org/x/mod/semver":
		return interp.handleModSemverCall(name, args)
	case "golang.org/x/mod/module":
		return interp.handleModModuleCall(name, args)
	case "golang.org/x/mod/modfile":
		return interp.handleModModfileCall(name, args)
	case "crypto":
		return interp.handleCryptoTopCall(name, args)
	case "testing/cryptotest":
		return interp.handleCryptoTestCall(name, args)
	// v0.63.0: x/text, x/term, x/crypto extras
	case "golang.org/x/text/cases":
		return interp.handleTextCasesCall(name, args)
	case "golang.org/x/text/language":
		return interp.handleTextLanguageCall(name, args)
	case "golang.org/x/text/transform":
		return interp.handleTextTransformCall(gid, site, name, args)
	case "golang.org/x/text/unicode/norm":
		return interp.handleTextNormCall(name, args)
	case "golang.org/x/text/width":
		return interp.handleTextWidthCall(name, args)
	case "golang.org/x/text/runes":
		return interp.handleTextRunesCall(gid, site, name, args)
	case "golang.org/x/term":
		return interp.handleTermCall(name, args)
	case "golang.org/x/crypto/chacha20poly1305":
		return interp.handleChacha20Call(name, args)
	case "golang.org/x/crypto/argon2":
		return interp.handleArgon2Call(name, args)
	case "golang.org/x/crypto/ssh":
		return interp.handleSSHCall(name, args)
	// v0.64.0: x/crypto NaCl, Blake2, ed25519; x/text encoding, collate, search
	case "golang.org/x/crypto/nacl/box":
		return interp.handleNaclBoxCall(name, args)
	case "golang.org/x/crypto/nacl/secretbox":
		return interp.handleNaclSecretboxCall(name, args)
	case "golang.org/x/crypto/curve25519":
		return interp.handleCurve25519Call(name, args)
	case "golang.org/x/crypto/poly1305":
		return interp.handlePoly1305Call(name, args)
	case "golang.org/x/crypto/blake2b":
		return interp.handleBlake2bCall(name, args)
	case "golang.org/x/crypto/blake2s":
		return interp.handleBlake2sCall(name, args)
	case "golang.org/x/crypto/ed25519":
		return interp.handleXEd25519Call(name, args)
	case "golang.org/x/text/encoding":
		return interp.handleTextEncodingCall(name, args)
	case "golang.org/x/text/encoding/charmap":
		return interp.handleTextCharmapCall(name, args)
	case "golang.org/x/text/encoding/unicode":
		return interp.handleTextEncodingUnicodeCall(name, args)
	case "golang.org/x/text/collate":
		return interp.handleTextCollateCall(name, args)
	case "golang.org/x/text/search":
		return interp.handleTextSearchCall(name, args)
	// v0.65.0: x/crypto stream ciphers + KDF; x/text message/number/currency/bidi/precis/encoding
	case "golang.org/x/crypto/scrypt":
		return interp.handleScryptCall(name, args)
	case "golang.org/x/crypto/chacha20":
		return interp.handleChacha20PrimCall(name, args)
	case "golang.org/x/crypto/xts":
		return interp.handleXTSCall(name, args)
	case "golang.org/x/crypto/salsa20":
		return interp.handleSalsa20Call(name, args)
	case "golang.org/x/text/message":
		return interp.handleTextMessageCall(name, args)
	case "golang.org/x/text/number":
		return interp.handleTextNumberCall(name, args)
	case "golang.org/x/text/currency":
		return interp.handleTextCurrencyCall(name, args)
	case "golang.org/x/text/unicode/bidi":
		return interp.handleTextBidiCall(name, args)
	case "golang.org/x/text/unicode/runenames":
		return interp.handleRuneNamesCall(name, args)
	case "golang.org/x/text/secure/bidirule":
		return interp.handleBidiRuleCall(name, args)
	case "golang.org/x/text/secure/precis":
		return interp.handlePrecisCall(name, args)
	case "golang.org/x/text/encoding/japanese":
		return interp.handleTextEncodingJapaneseCall(name, args)
	case "golang.org/x/text/encoding/htmlindex":
		return interp.handleTextHTMLIndexCall(name, args)
	// v0.66.0
	case "golang.org/x/text/encoding/korean":
		return interp.handleTextEncodingKoreanCall(name, args)
	case "golang.org/x/text/encoding/simplifiedchinese":
		return interp.handleTextEncodingSimplifiedChineseCall(name, args)
	case "golang.org/x/text/encoding/traditionalchinese":
		return interp.handleTextEncodingTraditionalChineseCall(name, args)
	case "golang.org/x/text/encoding/ianaindex":
		return interp.handleTextIANAIndexCall(name, args)
	case "golang.org/x/net/trace":
		return interp.handleNetTraceCall(name, args)
	case "golang.org/x/net/dns/dnsmessage":
		return interp.handleDNSMessageCall(name, args)
	case "golang.org/x/net/http2/hpack":
		return interp.handleHPACKCall(name, args)
	case "golang.org/x/sync/syncmap":
		return interp.handleSyncMapCall(name, args)
	}
	return Value{}, false
}

// handleGoPackagesCall intercepts golang.org/x/tools/go/packages calls (#148).
// packages.Load requires running "go list" via os/exec, which is not possible
// inside the interpreter. Return safe zero values to prevent false positives
// when analyzing programs that import go/packages (linters, code generators, etc.).
func (interp *Interpreter) handleGoPackagesCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Load":
		// Load([]*Package, error) — return empty slice and nil error.
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "NeedName", "NeedFiles", "NeedCompiledGoFiles", "NeedImports",
		"NeedDeps", "NeedTypes", "NeedSyntax", "NeedTypesInfo", "NeedTypesSizes":
		// LoadMode constants — return opaque non-zero value.
		return Value{Raw: int64(1)}, true
	}
	return Value{}, true // safe noop for anything else
}

// handleStringsCall models strings.* functions.
func (interp *Interpreter) handleStringsCall(name string, args []Value) (Value, bool) {
	s0, s0ok := stdlibArgString(args, 0)
	s1, s1ok := stdlibArgString(args, 1)

	switch name {
	case "Contains":
		if s0ok && s1ok {
			return Value{Raw: strings.Contains(s0, s1)}, true
		}
		return Value{Raw: true}, true // pessimistic: assume it contains
	case "HasPrefix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasPrefix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "HasSuffix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasSuffix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "EqualFold":
		if s0ok && s1ok {
			return Value{Raw: strings.EqualFold(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "ContainsAny", "ContainsRune":
		return Value{Raw: true}, true
	case "Index":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Index(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true // pessimistic: "found at position 0"
	case "LastIndex":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndex(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "IndexByte", "IndexRune", "IndexAny":
		return Value{Raw: int64(0)}, true
	case "Count":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Count(s0, s1))}, true
		}
		return Value{Raw: int64(1)}, true // pessimistic: "found 1 occurrence"
	case "TrimSpace":
		if s0ok {
			return Value{Raw: strings.TrimSpace(s0)}, true
		}
		return Value{Raw: "x"}, true
	case "Trim":
		if s0ok && s1ok {
			return Value{Raw: strings.Trim(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimLeft":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimLeft(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimRight":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimRight(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimPrefix":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimPrefix(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "TrimSuffix":
		if s0ok && s1ok {
			return Value{Raw: strings.TrimSuffix(s0, s1)}, true
		}
		return Value{Raw: "x"}, true
	case "ToUpper":
		if s0ok {
			return Value{Raw: strings.ToUpper(s0)}, true
		}
		return Value{Raw: "X"}, true
	case "ToLower":
		if s0ok {
			return Value{Raw: strings.ToLower(s0)}, true
		}
		return Value{Raw: "x"}, true
	case "ToTitle":
		if s0ok {
			return Value{Raw: strings.ToTitle(s0)}, true
		}
		return Value{Raw: "X"}, true
	case "Replace":
		s2, s2ok := stdlibArgString(args, 2)
		if s0ok && s1ok && s2ok {
			n := -1
			if nv, ok := stdlibArgInt(args, 3); ok {
				n = int(nv)
			}
			return Value{Raw: strings.Replace(s0, s1, s2, n)}, true
		}
		return Value{Raw: "x"}, true
	case "ReplaceAll":
		s2, s2ok := stdlibArgString(args, 2)
		if s0ok && s1ok && s2ok {
			return Value{Raw: strings.ReplaceAll(s0, s1, s2)}, true
		}
		return Value{Raw: "x"}, true
	case "Repeat":
		if s0ok {
			n := 1
			if nv, ok := stdlibArgInt(args, 1); ok {
				n = int(nv)
			}
			return Value{Raw: strings.Repeat(s0, n)}, true
		}
		return Value{Raw: "x"}, true
	case "Split":
		if s0ok && s1ok {
			return Value{Raw: stringsToValues(strings.Split(s0, s1))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "SplitN":
		if s0ok && s1ok {
			n, _ := stdlibArgInt(args, 2)
			return Value{Raw: stringsToValues(strings.SplitN(s0, s1, int(n)))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "SplitAfter":
		if s0ok && s1ok {
			return Value{Raw: stringsToValues(strings.SplitAfter(s0, s1))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "Fields":
		if s0ok {
			return Value{Raw: stringsToValues(strings.Fields(s0))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true
	case "Join":
		// args[0] is []string (stored as []Value), args[1] is sep.
		if s1ok {
			if sl, ok := args[0].Raw.([]Value); ok {
				parts := make([]string, len(sl))
				for i, v := range sl {
					if s, ok := v.Raw.(string); ok {
						parts[i] = s
					} else {
						parts[i] = "?"
					}
				}
				return Value{Raw: strings.Join(parts, s1)}, true
			}
		}
		return Value{Raw: "x"}, true
	case "Map":
		// args[0] is the mapping func (uninterpretable); args[1] is the string.
		// Return the string as-is (identity approximation).
		if s1ok {
			return Value{Raw: s1}, true
		}
		return Value{Raw: "x"}, true
	case "Compare":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Compare(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "Cut":
		if s0ok && s1ok {
			before, after, found := strings.Cut(s0, s1)
			return Value{Raw: []Value{{Raw: before}, {Raw: after}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}, {Raw: "x"}, {Raw: true}}}, true
	case "Clone":
		// strings.Clone(s) string — return same value (Go 1.20).
		if s0ok {
			return Value{Raw: s0}, true
		}
		return Value{Raw: "x"}, true

	case "CutPrefix":
		// strings.CutPrefix(s, prefix) (after string, found bool) (Go 1.20).
		if s0ok && s1ok {
			after, found := strings.CutPrefix(s0, s1)
			return Value{Raw: []Value{{Raw: after}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}, {Raw: false}}}, true

	case "CutSuffix":
		// strings.CutSuffix(s, suffix) (before string, found bool) (Go 1.20).
		if s0ok && s1ok {
			before, found := strings.CutSuffix(s0, s1)
			return Value{Raw: []Value{{Raw: before}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}, {Raw: false}}}, true

	case "ContainsFunc":
		// strings.ContainsFunc(s, f) bool (Go 1.21) — func not invoked; pessimistic.
		return Value{Raw: true}, true

	case "FieldsFunc":
		// strings.FieldsFunc(s, f) []string — func not invoked; return single-element.
		if s0ok {
			return Value{Raw: []Value{{Raw: s0}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true

	case "IndexFunc":
		// strings.IndexFunc(s, f) int — func not invoked; pessimistic 0.
		return Value{Raw: int64(0)}, true

	case "LastIndexAny":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndexAny(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true

	case "LastIndexByte":
		if s0ok {
			if c, ok := stdlibArgInt(args, 1); ok {
				return Value{Raw: int64(strings.LastIndexByte(s0, byte(c)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "LastIndexFunc":
		// strings.LastIndexFunc(s, f) int — func not invoked; pessimistic 0.
		return Value{Raw: int64(0)}, true

	case "SplitAfterN":
		if s0ok && s1ok {
			n, _ := stdlibArgInt(args, 2)
			return Value{Raw: stringsToValues(strings.SplitAfterN(s0, s1, int(n)))}, true
		}
		return Value{Raw: []Value{{Raw: "x"}}}, true

	case "Title":
		// strings.Title is deprecated but still present.
		if s0ok {
			return Value{Raw: strings.Title(s0)}, true //nolint:staticcheck
		}
		return Value{Raw: "X"}, true

	case "ToValidUTF8":
		if s0ok && s1ok {
			return Value{Raw: strings.ToValidUTF8(s0, s1)}, true
		}
		return Value{Raw: "x"}, true

	case "TrimFunc", "TrimLeftFunc", "TrimRightFunc":
		// func arg not invoked; return input string unchanged.
		if s0ok {
			return Value{Raw: s0}, true
		}
		return Value{Raw: "x"}, true

	case "NewReplacer":
		// Returns a *strings.Replacer (opaque but non-nil so method calls are dispatched).
		// Method calls (Replace, WriteString) share existing cases below.
		return Value{Raw: struct{}{}}, true

	// strings.Builder and *Replacer method calls (#79, #169): receiver = args[0], other args follow.
	case "WriteString":
		// b.WriteString(s) (int, error)
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "WriteByte":
		// b.WriteByte(c byte) error
		return Value{}, true
	case "WriteRune":
		// b.WriteRune(r rune) (int, error)
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true
	case "Write":
		// b.Write(p []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "String":
		// b.String() string
		return Value{Raw: ""}, true
	case "Len":
		// b.Len() int
		return Value{Raw: int64(0)}, true
	case "Cap":
		// b.Cap() int
		return Value{Raw: int64(0)}, true
	case "Reset":
		// b.Reset()
		return Value{}, true
	case "Grow":
		// b.Grow(n int)
		return Value{}, true

	// *strings.Reader constructors and methods (#103):
	case "NewReader":
		// strings.NewReader(s string) *strings.Reader — opaque non-nil.
		return Value{Raw: struct{}{}}, true

	// *strings.Reader methods (receiver = args[0]):
	case "Read", "ReadAt":
		// (int, error) — return (len(p), nil) pessimistically.
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "ReadByte":
		// (byte, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadRune":
		// (rune, int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(1)}, {}}}, true
	case "UnreadByte", "UnreadRune":
		return Value{}, true
	case "Seek":
		// (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Size":
		return Value{Raw: int64(0)}, true
	case "WriteTo":
		// (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	}
	return Value{}, false
}

// handleStrconvCall models strconv.* functions.
func (interp *Interpreter) handleStrconvCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Itoa":
		if n, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.Itoa(int(n))}, true
		}
		return Value{Raw: "0"}, true

	case "Atoi":
		if s, ok := stdlibArgString(args, 0); ok {
			n, err := strconv.Atoi(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: int64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
		}
		// Non-concrete: return sentinel success (1, nil).
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "FormatInt":
		if n, ok := stdlibArgInt(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			return Value{Raw: strconv.FormatInt(n, base)}, true
		}
		return Value{Raw: "0"}, true

	case "FormatUint":
		if n, ok := stdlibArgInt(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			return Value{Raw: strconv.FormatUint(uint64(n), base)}, true
		}
		return Value{Raw: "0"}, true

	case "FormatBool":
		if len(args) > 0 {
			if b, ok := args[0].Raw.(bool); ok {
				return Value{Raw: strconv.FormatBool(b)}, true
			}
		}
		return Value{Raw: "true"}, true

	case "ParseInt":
		if s, ok := stdlibArgString(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 2); ok2 {
				bitSize = int(bs)
			}
			n, err := strconv.ParseInt(s, base, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: int64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: n}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "ParseUint":
		if s, ok := stdlibArgString(args, 0); ok {
			base := 10
			if b, ok2 := stdlibArgInt(args, 1); ok2 {
				base = int(b)
			}
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 2); ok2 {
				bitSize = int(bs)
			}
			n, err := strconv.ParseUint(s, base, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: uint64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "ParseFloat":
		if s, ok := stdlibArgString(args, 0); ok {
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 1); ok2 {
				bitSize = int(bs)
			}
			f, err := strconv.ParseFloat(s, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: float64(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: f}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(1.0)}, {}}}, true

	case "ParseBool":
		if s, ok := stdlibArgString(args, 0); ok {
			b, err := strconv.ParseBool(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: false}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: b}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: true}, {}}}, true

	case "FormatFloat":
		if f, ok := stdlibArgFloat(args, 0); ok {
			fmtByte := byte('g')
			if len(args) > 1 {
				switch v := args[1].Raw.(type) {
				case int32:
					fmtByte = byte(v)
				case int64:
					fmtByte = byte(v)
				}
			}
			prec := -1
			if p, ok2 := stdlibArgInt(args, 2); ok2 {
				prec = int(p)
			}
			bitSize := 64
			if bs, ok2 := stdlibArgInt(args, 3); ok2 {
				bitSize = int(bs)
			}
			return Value{Raw: strconv.FormatFloat(f, fmtByte, prec, bitSize)}, true
		}
		return Value{Raw: "0"}, true

	case "Quote":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strconv.Quote(s)}, true
		}
		return Value{Raw: `"x"`}, true

	case "Unquote":
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := strconv.Unquote(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "x"}, {}}}, true

	case "AppendInt", "AppendUint", "AppendFloat", "AppendBool", "AppendQuote",
		"AppendQuoteRune", "AppendQuoteRuneToASCII", "AppendQuoteRuneToGraphic",
		"AppendQuoteToASCII", "AppendQuoteToGraphic":
		// Returns []byte; return the dst slice unchanged.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "QuoteRune":
		// strconv.QuoteRune(r rune) string
		if r, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.QuoteRune(rune(r))}, true
		}
		return Value{Raw: `'x'`}, true

	case "QuoteRuneToASCII":
		if r, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.QuoteRuneToASCII(rune(r))}, true
		}
		return Value{Raw: `'x'`}, true

	case "QuoteRuneToGraphic":
		if r, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.QuoteRuneToGraphic(rune(r))}, true
		}
		return Value{Raw: `'x'`}, true

	case "QuoteToASCII":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strconv.QuoteToASCII(s)}, true
		}
		return Value{Raw: `"x"`}, true

	case "QuoteToGraphic":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strconv.QuoteToGraphic(s)}, true
		}
		return Value{Raw: `"x"`}, true

	case "QuotedPrefix":
		// strconv.QuotedPrefix(s string) (string, error) — Go 1.17
		if s, ok := stdlibArgString(args, 0); ok {
			p, err := strconv.QuotedPrefix(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: p}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: `"x"`}, {}}}, true

	case "CanBackquote":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strconv.CanBackquote(s)}, true
		}
		return Value{Raw: true}, true

	case "IsPrint":
		if r, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.IsPrint(rune(r))}, true
		}
		return Value{Raw: true}, true

	case "IsGraphic":
		if r, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: strconv.IsGraphic(rune(r))}, true
		}
		return Value{Raw: true}, true

	case "ParseComplex":
		// strconv.ParseComplex(s string, bitSize int) (complex128, error)
		if s, ok := stdlibArgString(args, 0); ok {
			bitSize := 128
			if bs, ok2 := stdlibArgInt(args, 1); ok2 {
				bitSize = int(bs)
			}
			c, err := strconv.ParseComplex(s, bitSize)
			if err != nil {
				return Value{Raw: []Value{{Raw: complex128(0)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: c}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: complex128(1 + 0i)}, {}}}, true

	case "FormatComplex":
		// strconv.FormatComplex(c complex128, fmt byte, prec, bitSize int) string
		if len(args) >= 4 {
			if c, ok := args[0].Raw.(complex128); ok {
				fmtByte := byte('g')
				if f, ok2 := stdlibArgInt(args, 1); ok2 {
					fmtByte = byte(f)
				}
				prec := -1
				if p, ok2 := stdlibArgInt(args, 2); ok2 {
					prec = int(p)
				}
				bitSize := 128
				if bs, ok2 := stdlibArgInt(args, 3); ok2 {
					bitSize = int(bs)
				}
				return Value{Raw: strconv.FormatComplex(c, fmtByte, prec, bitSize)}, true
			}
		}
		return Value{Raw: "(1+0i)"}, true

	case "UnquoteChar":
		// strconv.UnquoteChar(s string, quote byte) (rune, bool, string, error)
		if s, ok := stdlibArgString(args, 0); ok {
			q := byte('"')
			if qv, ok2 := stdlibArgInt(args, 1); ok2 {
				q = byte(qv)
			}
			r, multi, tail, err := strconv.UnquoteChar(s, q)
			if err != nil {
				return Value{Raw: []Value{{Raw: int64(r)}, {Raw: multi}, {Raw: tail}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: int64(r)}, {Raw: multi}, {Raw: tail}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: int64('x')}, {Raw: false}, {Raw: ""}, {}}}, true
	}
	return Value{}, false
}

// handleFmtCall models fmt.* functions.
func (interp *Interpreter) handleFmtCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Sprintf":
		if len(args) == 0 {
			return Value{Raw: ""}, true
		}
		if format, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: fmt.Sprintf(format, valuesToInterfaces(args[1:])...)}, true
		}
		return Value{Raw: "<fmt.Sprintf>"}, true

	case "Errorf":
		if len(args) == 0 {
			return Value{Raw: fmt.Errorf("<fmt.Errorf>")}, true
		}
		if format, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: fmt.Errorf(format, valuesToInterfaces(args[1:])...)}, true
		}
		return Value{Raw: fmt.Errorf("<fmt.Errorf>")}, true

	case "Sprint":
		return Value{Raw: fmt.Sprint(valuesToInterfaces(args)...)}, true

	case "Sprintln":
		return Value{Raw: fmt.Sprintln(valuesToInterfaces(args)...)}, true

	case "Println", "Printf", "Print", "Fprintln", "Fprintf", "Fprint":
		// Return (1, nil): 1 byte written, nil error (#65).
		// Callers checking err != nil take the non-error path; callers
		// checking n == 0 see a plausible non-zero byte count.
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "Sscanf", "Sscan", "Sscanln":
		// Return (0, nil): 0 items scanned, nil error.
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "Scan", "Scanf", "Scanln":
		// fmt.Scan/Scanf/Scanln read from os.Stdin — return (0, nil) (#161).
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "Fscan", "Fscanf", "Fscanln":
		// fmt.Fscan/Fscanf/Fscanln read from an io.Reader — return (0, nil) (#161).
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	}
	return Value{}, false
}

// stdlibArgString extracts a concrete string Value at index i.
func stdlibArgString(args []Value, i int) (string, bool) {
	if i >= len(args) {
		return "", false
	}
	s, ok := args[i].Raw.(string)
	return s, ok
}

// stdlibArgInt extracts a concrete integer Value at index i.
// Recognizes the int family that toInt64 handles.
func stdlibArgInt(args []Value, i int) (int64, bool) {
	if i >= len(args) {
		return 0, false
	}
	switch args[i].Raw.(type) {
	case int64, int, int32, int16, int8, uint64, uint32, uint16, uint8, uint:
		return toInt64(args[i]), true
	}
	return 0, false
}

// stdlibArgFloat extracts a concrete float Value at index i.
func stdlibArgFloat(args []Value, i int) (float64, bool) {
	if i >= len(args) {
		return 0, false
	}
	switch v := args[i].Raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// stringsToValues converts a []string to []Value.
func stringsToValues(ss []string) []Value {
	vs := make([]Value, len(ss))
	for i, s := range ss {
		vs[i] = Value{Raw: s}
	}
	return vs
}

// handleTimeCall models time.* functions (#45, #93).
// time.After returns a channel that immediately has a value (simulates a fired timer).
// time.Sleep is a noop. NewTicker/NewTimer return opaque values.
func (interp *Interpreter) handleTimeCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "After":
		if len(args) >= 2 {
			// (time.Time).After(u time.Time) bool — method call with receiver+arg.
			return Value{Raw: false}, true
		}
		// time.After(d Duration) <-chan Time — package-level.
		chanID := interp.createChannel(1)
		if ch, ok := interp.channels[chanID]; ok {
			ch.hasPending = true
			ch.pendingCount = 1
		}
		interp.channelSenders[chanID] = true
		return Value{Raw: chanID}, true

	case "Tick":
		// time.Tick(d) <-chan Time — like After but never explicitly stopped.
		chanID := interp.createChannel(1)
		if ch, ok := interp.channels[chanID]; ok {
			ch.hasPending = true
			ch.pendingCount = 1
		}
		interp.channelSenders[chanID] = true
		return Value{Raw: chanID}, true

	case "Sleep":
		// Noop — goroutine continues immediately; no side effects to model.
		return Value{}, true

	case "NewTicker":
		// time.NewTicker(d) *Ticker — return opaque; Stop/Reset dispatched via same intercept.
		return opaque, true

	case "NewTimer":
		// time.NewTimer(d) *Timer — return opaque; Stop/Reset dispatched via same intercept.
		return opaque, true

	case "Now":
		// time.Now() time.Time — return opaque time.Time.
		return opaque, true

	case "Since", "Until":
		// Returns time.Duration (int64 nanoseconds). Return 1ns so downstream
		// comparisons against 0 see a non-zero value.
		return Value{Raw: int64(1)}, true

	case "Unix", "UnixMicro", "UnixMilli":
		if len(args) <= 1 {
			// Method: (time.Time).Unix() int64 / .UnixMicro() / .UnixMilli()
			return Value{Raw: int64(0)}, true
		}
		// Package-level: time.Unix(sec, nsec) time.Time
		return opaque, true

	case "UnixNano":
		// Only exists as a method: (time.Time).UnixNano() int64
		return Value{Raw: int64(0)}, true

	case "Date":
		// time.Date(...) time.Time
		return opaque, true

	case "ParseDuration":
		// Returns (time.Duration, error) — return (1ns, nil).
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	case "Parse", "ParseInLocation":
		// Returns (time.Time, error) — return (opaque, nil).
		return Value{Raw: []Value{opaque, {}}}, true

	// time.Time methods (receiver in args[0]):
	case "Add", "Round", "Truncate", "In", "UTC", "Local":
		return opaque, true

	case "Sub":
		// (time.Time).Sub(time.Time) time.Duration
		return Value{Raw: int64(0)}, true

	case "Before", "Equal", "IsZero":
		return Value{Raw: false}, true

	case "Format", "String":
		return Value{Raw: ""}, true

	case "Year", "Month", "Day", "Hour", "Minute", "Second",
		"Nanosecond", "YearDay", "Weekday":
		return Value{Raw: int64(0)}, true

	case "Zone":
		// Returns (name string, offset int).
		return Value{Raw: []Value{{Raw: ""}, {Raw: int64(0)}}}, true

	case "MarshalJSON", "MarshalText", "MarshalBinary":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true

	case "UnmarshalJSON", "UnmarshalText", "UnmarshalBinary":
		return Value{}, true

	// Ticker / Timer methods:
	case "Stop":
		// (*Ticker).Stop() and (*Timer).Stop() return bool.
		return Value{Raw: false}, true

	case "Reset":
		// (*Ticker).Reset(d) and (*Timer).Reset(d) return bool.
		return Value{Raw: false}, true

	// time.Duration methods:
	case "Hours", "Minutes", "Seconds":
		return Value{Raw: float64(0)}, true

	case "Milliseconds", "Microseconds", "Nanoseconds":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, false
}

// handleOSCall models os.* functions (#62, #94).
// os.Exit is handled separately in execCall (it needs to stop all goroutines).
// This intercept covers environment, filesystem queries, and *os.File methods.
func (interp *Interpreter) handleOSCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Getenv":
		// Return empty string for any env var — conservative but safe.
		return Value{Raw: ""}, true
	case "LookupEnv":
		// Return ("", false): key not found.
		return Value{Raw: []Value{{Raw: ""}, {Raw: false}}}, true
	case "Setenv", "Unsetenv":
		// Return nil error.
		return Value{}, true
	case "Getwd":
		// Return ("/tmp", nil) — a valid directory path.
		return Value{Raw: []Value{{Raw: "/tmp"}, {}}}, true
	case "MkdirAll", "MkdirTemp", "Mkdir", "Remove", "RemoveAll", "Rename":
		// File-system mutations: noop with nil error.
		return Value{}, true
	case "Open", "Create", "CreateTemp", "OpenFile":
		// Return (opaque *os.File, nil) — opaque so that method calls on the
		// returned file are dispatched back to this intercept (#94).
		return Value{Raw: []Value{opaque, {}}}, true
	case "ReadFile", "WriteFile":
		// Bulk I/O: return ([]byte{}, nil).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true

	// *os.File methods — receiver in args[0], payload in args[1:].
	case "Read", "ReadAt":
		// (n int, err error) — return (len(p), nil) pessimistically.
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "Write", "WriteAt":
		// (n int, err error) — return (len(p), nil).
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "WriteString":
		// (n int, err error) — return (len(s), nil).
		n := int64(0)
		if s, ok := stdlibArgString(args, 1); ok {
			n = int64(len(s))
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "Close":
		return Value{}, true
	case "Stat":
		// (os.FileInfo, error) — return (opaque, nil).
		return Value{Raw: []Value{opaque, {}}}, true
	case "Seek":
		// (int64, error) — return (0, nil).
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Sync", "Chmod", "Chown", "Lchown", "Truncate", "Chdir":
		return Value{}, true
	case "Name":
		return Value{Raw: ""}, true
	case "Fd":
		// Return 3 (first non-standard fd).
		return Value{Raw: int64(3)}, true
	case "ReadDir":
		// Return ([]os.DirEntry{}, nil).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Readdirnames":
		// Return ([]string{}, nil).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Readdir":
		// Return ([]os.FileInfo{}, nil).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "DirFS":
		// DirFS(dir string) fs.FS — return opaque fs.FS value.
		return opaque, true

	// --- Added in v0.69.0 ---

	case "Lstat":
		// Lstat(name string) (FileInfo, error) — same semantics as Stat.
		return Value{Raw: []Value{opaque, {}}}, true
	case "TempDir":
		// TempDir() string — return the OS temporary directory.
		return Value{Raw: "/tmp"}, true
	case "Hostname":
		// Hostname() (string, error)
		return Value{Raw: []Value{{Raw: "localhost"}, {}}}, true
	case "Getpid":
		return Value{Raw: int64(os.Getpid())}, true
	case "Getuid":
		return Value{Raw: int64(os.Getuid())}, true
	case "Getgid":
		return Value{Raw: int64(os.Getgid())}, true
	case "Geteuid":
		return Value{Raw: int64(os.Geteuid())}, true
	case "Getegid":
		return Value{Raw: int64(os.Getegid())}, true
	case "Getgroups":
		// Getgroups() ([]int, error) — return empty slice, nil error.
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "IsNotExist", "IsExist", "IsPermission", "IsTimeout":
		// Conservative: return false (unknown error type).
		return Value{Raw: false}, true
	case "ExpandEnv":
		// ExpandEnv(s string) string — return empty string (env not modeled).
		return Value{Raw: ""}, true
	case "Environ":
		// Environ() []string — return empty slice.
		return Value{Raw: []Value{}}, true
	case "Clearenv":
		return Value{}, true
	case "Executable":
		// Executable() (string, error)
		return Value{Raw: []Value{{Raw: "/tmp/giri-test"}, {}}}, true
	case "UserHomeDir", "UserCacheDir", "UserConfigDir":
		// (string, error)
		return Value{Raw: []Value{{Raw: "/tmp"}, {}}}, true
	case "Pipe":
		// Pipe() (*File, *File, error) — return two opaque files, nil error.
		return Value{Raw: []Value{opaque, opaque, {}}}, true
	case "Readlink":
		// Readlink(name string) (string, error)
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "Link", "Symlink", "Chtimes":
		// File-system operations that return only error.
		return Value{}, true
	case "SameFile":
		// SameFile(fi1, fi2 FileInfo) bool — conservative false.
		return Value{Raw: false}, true
	case "Exit":
		// os.Exit terminates the process; in the interpreter, noop.
		return Value{}, true
	}
	return Value{}, false
}

// handleMathRandCall models math/rand.* functions (#64).
// Uses the interpreter's per-run RNG (seeded from config.RandomSeed) for
// deterministic, reproducible values without interpreting stdlib internals.
func (interp *Interpreter) handleMathRandCall(name string, args []Value) (Value, bool) {
	rng := interp.rng
	if rng == nil {
		rng = rand.New(rand.NewSource(0))
	}
	switch name {
	case "Intn", "Int31n":
		if n, ok := stdlibArgInt(args, 0); ok && n > 0 {
			return Value{Raw: rng.Int63n(n)}, true
		}
		return Value{Raw: int64(0)}, true
	case "Int63n":
		if n, ok := stdlibArgInt(args, 0); ok && n > 0 {
			return Value{Raw: rng.Int63n(n)}, true
		}
		return Value{Raw: int64(0)}, true
	case "Int63":
		return Value{Raw: rng.Int63()}, true
	case "Int", "Int31":
		return Value{Raw: int64(rng.Int31())}, true
	case "Uint32":
		return Value{Raw: int64(rng.Uint32())}, true
	case "Uint64":
		return Value{Raw: int64(rng.Uint64())}, true
	case "Float64":
		return Value{Raw: rng.Float64()}, true
	case "Float32":
		return Value{Raw: float64(rng.Float32())}, true
	case "Seed":
		// Noop: we control the seed via config.RandomSeed.
		return Value{}, true
	case "New":
		// Return opaque Value — method calls on the returned *Rand are handled
		// via the same math/rand intercept (same package path).
		return Value{}, true
	case "NewSource":
		return Value{}, true
	case "Perm":
		if n, ok := stdlibArgInt(args, 0); ok && n >= 0 {
			vs := make([]Value, n)
			for i := range vs {
				vs[i] = Value{Raw: int64(i)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{}}, true
	case "Shuffle":
		// Noop: element ordering doesn't affect memory safety.
		return Value{}, true
	case "Read":
		// Return (n, nil) where n = length of the slice arg (if known).
		n := int64(0)
		if len(args) > 0 {
			if sv, ok := args[0].Raw.(*SliceValue); ok {
				n = int64(sv.Len)
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "NormFloat64", "ExpFloat64":
		return Value{Raw: rng.NormFloat64()}, true
	}
	return Value{}, false
}

// handleBytesCall models bytes.* functions (#66).
// Mirrors handleStringsCall: for concrete byte-slice arguments the equivalent
// strings function is used (treating []byte as string); for opaque arguments
// conservative/pessimistic values are returned.
func (interp *Interpreter) handleBytesCall(name string, args []Value) (Value, bool) {
	// Helper: extract bytes arg as string (best-effort; opaque if not concrete).
	asStr := func(i int) (string, bool) {
		if i >= len(args) {
			return "", false
		}
		switch v := args[i].Raw.(type) {
		case string:
			return v, true
		case []byte:
			return string(v), true
		}
		return "", false
	}
	s0, s0ok := asStr(0)
	s1, s1ok := asStr(1)

	switch name {
	// --- Predicates (return true conservatively when args are opaque) ---
	case "Contains":
		if s0ok && s1ok {
			return Value{Raw: strings.Contains(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "ContainsAny":
		if s0ok && s1ok {
			return Value{Raw: strings.ContainsAny(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "ContainsRune":
		return Value{Raw: true}, true
	case "HasPrefix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasPrefix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "HasSuffix":
		if s0ok && s1ok {
			return Value{Raw: strings.HasSuffix(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "Equal":
		if s0ok && s1ok {
			return Value{Raw: s0 == s1}, true
		}
		return Value{Raw: true}, true
	case "EqualFold":
		if s0ok && s1ok {
			return Value{Raw: strings.EqualFold(s0, s1)}, true
		}
		return Value{Raw: true}, true
	case "Compare":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Compare(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true

	// --- Index functions (return 0 / non-negative for opaque args) ---
	case "Count":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Count(s0, s1))}, true
		}
		return Value{Raw: int64(1)}, true
	case "Index":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.Index(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "IndexAny":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.IndexAny(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "IndexByte":
		return Value{Raw: int64(0)}, true
	case "IndexRune":
		return Value{Raw: int64(0)}, true
	case "LastIndex":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndex(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true
	case "LastIndexAny":
		if s0ok && s1ok {
			return Value{Raw: int64(strings.LastIndexAny(s0, s1))}, true
		}
		return Value{Raw: int64(0)}, true

	// --- Transforming functions (return input unchanged for opaque args) ---
	case "ToLower":
		if s0ok {
			return Value{Raw: []byte(strings.ToLower(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "ToUpper":
		if s0ok {
			return Value{Raw: []byte(strings.ToUpper(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "ToTitle":
		if s0ok {
			return Value{Raw: []byte(strings.ToTitle(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Title":
		if s0ok {
			return Value{Raw: []byte(strings.ToTitle(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "TrimSpace":
		if s0ok {
			return Value{Raw: []byte(strings.TrimSpace(s0))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Trim", "TrimLeft", "TrimRight":
		if s0ok {
			return Value{Raw: args[0].Raw}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "TrimFunc", "TrimLeftFunc", "TrimRightFunc", "Map":
		return Value{Raw: args[0].Raw}, true
	case "TrimPrefix":
		if s0ok && s1ok {
			return Value{Raw: []byte(strings.TrimPrefix(s0, s1))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "TrimSuffix":
		if s0ok && s1ok {
			return Value{Raw: []byte(strings.TrimSuffix(s0, s1))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Replace":
		if s0ok && s1ok {
			s2, _ := asStr(2)
			n := -1
			if len(args) >= 4 {
				n = int(toInt64(args[3]))
			}
			return Value{Raw: []byte(strings.Replace(s0, s1, s2, n))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "ReplaceAll":
		if s0ok && s1ok {
			s2, _ := asStr(2)
			return Value{Raw: []byte(strings.ReplaceAll(s0, s1, s2))}, true
		}
		return Value{Raw: args[0].Raw}, true
	case "Repeat":
		if s0ok {
			n := 1
			if len(args) >= 2 {
				n = int(toInt64(args[1]))
			}
			return Value{Raw: []byte(strings.Repeat(s0, n))}, true
		}
		return Value{Raw: args[0].Raw}, true

	// --- Splitting functions (return single-element slice for opaque args) ---
	case "Split", "SplitN", "SplitAfter", "SplitAfterN":
		if s0ok && s1ok {
			parts := strings.Split(s0, s1)
			vs := make([]Value, len(parts))
			for i, p := range parts {
				vs[i] = Value{Raw: []byte(p)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{args[0]}}, true
	case "Fields":
		if s0ok {
			parts := strings.Fields(s0)
			vs := make([]Value, len(parts))
			for i, p := range parts {
				vs[i] = Value{Raw: []byte(p)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{args[0]}}, true
	case "Join":
		if len(args) >= 1 {
			return Value{Raw: args[0].Raw}, true
		}
		return Value{Raw: []byte{}}, true
	case "Cut":
		if s0ok && s1ok {
			before, after, found := strings.Cut(s0, s1)
			return Value{Raw: []Value{{Raw: []byte(before)}, {Raw: []byte(after)}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{args[0], {Raw: []byte{}}, {Raw: false}}}, true
	case "CutPrefix":
		if s0ok && s1ok {
			after, found := strings.CutPrefix(s0, s1)
			return Value{Raw: []Value{{Raw: []byte(after)}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{args[0], {Raw: false}}}, true
	case "CutSuffix":
		if s0ok && s1ok {
			before, found := strings.CutSuffix(s0, s1)
			return Value{Raw: []Value{{Raw: []byte(before)}, {Raw: found}}}, true
		}
		return Value{Raw: []Value{args[0], {Raw: false}}}, true
	case "Clone":
		return Value{Raw: args[0].Raw}, true

	case "ContainsFunc":
		// bytes.ContainsFunc(b []byte, f func(rune) bool) bool — func not invoked.
		return Value{Raw: true}, true

	case "FieldsFunc":
		// bytes.FieldsFunc(b []byte, f func(rune) bool) [][]byte — func not invoked.
		return Value{Raw: []Value{{Raw: args[0].Raw}}}, true

	case "IndexFunc":
		// bytes.IndexFunc(b []byte, f func(rune) bool) int — func not invoked.
		return Value{Raw: int64(0)}, true

	case "LastIndexByte":
		// bytes.LastIndexByte(b []byte, c byte) int
		if s0ok {
			if c, ok := stdlibArgInt(args, 1); ok {
				idx := strings.LastIndexByte(s0, byte(c))
				return Value{Raw: int64(idx)}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "LastIndexFunc":
		// bytes.LastIndexFunc(b []byte, f func(rune) bool) int — func not invoked.
		return Value{Raw: int64(0)}, true

	case "Runes":
		// bytes.Runes(b []byte) []rune
		if s0ok {
			rs := []rune(s0)
			vs := make([]Value, len(rs))
			for i, r := range rs {
				vs[i] = Value{Raw: int64(r)}
			}
			return Value{Raw: vs}, true
		}
		return Value{Raw: []Value{}}, true

	case "ToValidUTF8":
		if s0ok && s1ok {
			return Value{Raw: []byte(strings.ToValidUTF8(s0, s1))}, true
		}
		return Value{Raw: args[0].Raw}, true

	// bytes.Buffer method calls (#79): receiver = args[0], other args follow.
	case "WriteString":
		// buf.WriteString(s) (int, error)
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "WriteByte":
		// buf.WriteByte(c byte) error
		return Value{}, true
	case "WriteRune":
		// buf.WriteRune(r rune) (int, error)
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true
	case "Write":
		// buf.Write(p []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "String":
		// buf.String() string
		return Value{Raw: ""}, true
	case "Bytes":
		// buf.Bytes() []byte
		return Value{Raw: []byte(nil)}, true
	case "Len":
		// buf.Len() int
		return Value{Raw: int64(0)}, true
	case "Cap":
		// buf.Cap() int
		return Value{Raw: int64(0)}, true
	case "Reset":
		// buf.Reset()
		return Value{}, true
	case "Truncate":
		// buf.Truncate(n int)
		return Value{}, true
	case "Grow":
		// buf.Grow(n int)
		return Value{}, true
	case "ReadFrom":
		// buf.ReadFrom(r io.Reader) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "WriteTo":
		// buf.WriteTo(w io.Writer) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadByte":
		// buf.ReadByte() (byte, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadRune":
		// buf.ReadRune() (rune, int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {}}}, true
	case "ReadString":
		// buf.ReadString(delim byte) (string, error)
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "ReadBytes":
		// buf.ReadBytes(delim byte) ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "Next":
		// buf.Next(n int) []byte
		return Value{Raw: []byte(nil)}, true
	case "UnreadByte":
		// buf.UnreadByte() error
		return Value{}, true
	case "UnreadRune":
		// buf.UnreadRune() error
		return Value{}, true

	// bytes.NewBuffer / bytes.NewBufferString / bytes.NewReader constructors (#103):
	case "NewBuffer", "NewBufferString":
		// bytes.NewBuffer(buf []byte) *bytes.Buffer — opaque non-nil.
		return Value{Raw: struct{}{}}, true
	case "NewReader":
		// bytes.NewReader(b []byte) *bytes.Reader — opaque non-nil.
		return Value{Raw: struct{}{}}, true

	// *bytes.Reader methods (receiver = args[0]):
	case "Seek":
		// (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Size":
		return Value{Raw: int64(0)}, true
	case "Read":
		// (int, error) — returns (len(p), nil)
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "ReadAt":
		// (int, error)
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	}
	return Value{}, false
}

// handleErrorsCall models errors.* functions (#67).
func (interp *Interpreter) handleErrorsCall(name string, args []Value) (Value, bool) {
	switch name {
	case "New":
		msg := "<error>"
		if s, ok := stdlibArgString(args, 0); ok {
			msg = s
		}
		return Value{Raw: fmt.Errorf("%s", msg)}, true //nolint:err113

	case "Is":
		// Conservative: compare error strings if both are concrete errors.
		if len(args) >= 2 {
			if args[0].Raw == nil && args[1].Raw == nil {
				return Value{Raw: false}, true
			}
			e1, ok1 := args[0].Raw.(error)
			e2, ok2 := args[1].Raw.(error)
			if ok1 && ok2 {
				return Value{Raw: e1.Error() == e2.Error()}, true
			}
			// One or both not concrete — conservatively return false (no match).
		}
		return Value{Raw: false}, true

	case "As":
		// Conservative: always return false (no unwrapping chain modeled).
		return Value{Raw: false}, true

	case "Unwrap":
		// Conservative: no wrapping chain; return nil.
		return Value{}, true

	case "Join":
		// errors.Join(errs ...error) — return first non-nil if any.
		for _, a := range args {
			if a.Raw != nil {
				return Value{Raw: a.Raw}, true
			}
		}
		return Value{}, true
	}
	return Value{}, false
}

// handleSortCall models sort.* functions (#68).
// For functions that accept a user callback (less func, f func), the callback
// is probed once with representative arguments to surface any violations in it.
func (interp *Interpreter) handleSortCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback invokes a function-value arg at position argIdx with the
	// given explicit callArgs. For closures, FreeVars follow the explicit params
	// (matching how execFunction binds free variables: params[0..n-1] then freeVars[0..m-1]).
	probeCallback := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			// Params come first, then free vars (see execFunction param binding).
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	switch name {
	case "Slice", "SliceStable":
		// Probe comparator with (0, 1) to detect violations in the callback.
		probeCallback(1, []Value{{Raw: int64(0)}, {Raw: int64(1)}})
		return Value{}, true

	case "SliceIsSorted":
		probeCallback(1, []Value{{Raw: int64(0)}, {Raw: int64(1)}})
		return Value{Raw: true}, true

	case "Search":
		// sort.Search(n int, f func(int) bool) int: probe f with n/2.
		n := int64(0)
		if len(args) > 0 {
			n = toInt64(args[0])
		}
		mid := n / 2
		probeCallback(1, []Value{{Raw: mid}})
		return Value{Raw: mid}, true

	case "Strings", "Ints", "Float64s":
		// Noop — sort in-place, no memory safety implications.
		return Value{}, true

	case "Sort", "Stable", "Reverse", "IsSorted":
		// sort.Interface operations — noop.
		return Value{}, true

	case "Find":
		// sort.Find(n int, cmp func(int) int) (int, bool): probe cmp with 0.
		probeCallback(1, []Value{{Raw: int64(0)}})
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true
	}
	return Value{}, false
}

// handleJSONCall models encoding/json.* functions (#70).
// Marshal and MarshalIndent return ([]byte, nil). Unmarshal returns nil error.
// Decoder/Encoder creation and methods return opaque or nil values.
func (interp *Interpreter) handleJSONCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Marshal", "MarshalIndent":
		// Returns ([]byte, error). Return non-nil bytes so callers checking len(b) > 0 succeed.
		return Value{Raw: []Value{{Raw: []byte(`null`)}, {}}}, true

	case "Unmarshal":
		// Returns error (nil). We don't model the actual deserialization into the target.
		return Value{}, true

	case "NewDecoder", "NewEncoder":
		// Return an opaque value so method calls on the result are intercepted
		// via the same "encoding/json" package path.
		return Value{Raw: struct{}{}}, true

	case "Decode", "Encode":
		// Decoder.Decode / Encoder.Encode: return nil error.
		return Value{}, true

	case "More":
		// Conservative: always more tokens.
		return Value{Raw: true}, true

	case "Token":
		// Returns (Token, error): return ("", nil).
		return Value{Raw: []Value{{Raw: ""}, {}}}, true

	case "Valid":
		return Value{Raw: true}, true

	case "Compact", "Indent", "HTMLEscape":
		// Returns error (nil for Compact/Indent); HTMLEscape is void.
		return Value{}, true

	case "Number":
		return Value{Raw: ""}, true

	// Decoder option setters (return the decoder receiver for chaining; noop here):
	case "UseNumber", "DisallowUnknownFields":
		return Value{}, true

	// Decoder informational methods:
	case "InputOffset":
		// Decoder.InputOffset() int64 — bytes consumed so far.
		return Value{Raw: int64(0)}, true
	case "Buffered":
		// Decoder.Buffered() io.Reader — remaining buffered data.
		return stdlibOpaque, true

	// Encoder option setters:
	case "SetIndent", "SetEscapeHTML":
		return Value{}, true
	}
	return Value{}, false
}

// handleRegexpCall models regexp.* package-level functions and *Regexp methods (#71).
// For package-level Match* that return (bool, error), a tuple is returned.
// For method calls (receiver = args[0] is an opaque Regexp), a plain bool is returned.
// The two cases are distinguished by whether args[0] is a string (package-level pattern).
func (interp *Interpreter) handleRegexpCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback invokes a function-value at position argIdx with the given callArgs.
	probeCallback := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	// isPackageLevel: true when args[0] is a string (the pattern argument of
	// regexp.MatchString/Match/etc.) rather than a *Regexp receiver.
	isPackageLevel := len(args) > 0
	if len(args) > 0 {
		_, isPackageLevel = args[0].Raw.(string)
	}

	switch name {
	case "Compile":
		// (expr string) (*Regexp, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "MustCompile":
		// (expr string) *Regexp
		return Value{Raw: struct{}{}}, true

	case "Match":
		if isPackageLevel {
			return Value{Raw: []Value{{Raw: true}, {}}}, true
		}
		return Value{Raw: true}, true

	case "MatchString":
		if isPackageLevel {
			return Value{Raw: []Value{{Raw: true}, {}}}, true
		}
		return Value{Raw: true}, true

	case "MatchReader":
		if isPackageLevel {
			return Value{Raw: []Value{{Raw: true}, {}}}, true
		}
		return Value{Raw: true}, true

	case "QuoteMeta":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: regexp_quoteMeta(s)}, true
		}
		return Value{Raw: ""}, true

	case "FindString":
		// receiver = args[0], src = args[1]
		return Value{Raw: ""}, true

	case "FindStringIndex":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}}}, true

	case "FindStringSubmatch":
		return Value{Raw: []Value{}}, true

	case "FindAllString":
		return Value{Raw: []Value{}}, true

	case "FindAllStringSubmatch":
		return Value{Raw: []Value{}}, true

	case "FindAllStringIndex":
		return Value{Raw: []Value{}}, true

	case "ReplaceAllString":
		// receiver = args[0], src = args[1], repl = args[2]; return src unchanged.
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true

	case "ReplaceAllLiteralString":
		// Return the replacement string.
		if len(args) > 2 {
			if s, ok := args[2].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true

	case "ReplaceAllStringFunc":
		// Probe the callback with "" to surface violations inside it.
		probeCallback(2, []Value{{Raw: ""}})
		if len(args) > 1 {
			return args[1], true
		}
		return Value{Raw: ""}, true

	case "ReplaceAll":
		if len(args) > 1 {
			return args[1], true
		}
		return Value{Raw: []byte{}}, true

	case "ReplaceAllLiteral":
		if len(args) > 2 {
			return args[2], true
		}
		return Value{Raw: []byte{}}, true

	case "Split":
		if len(args) > 1 {
			return Value{Raw: []Value{args[1]}}, true
		}
		return Value{Raw: []Value{}}, true

	case "SubexpNames":
		return Value{Raw: []Value{}}, true

	case "SubexpIndex":
		return Value{Raw: int64(-1)}, true

	case "NumSubexp":
		return Value{Raw: int64(0)}, true

	case "String":
		return Value{Raw: ""}, true

	case "Longest", "Copy":
		return Value{Raw: struct{}{}}, true

	case "Find", "FindIndex", "FindSubmatch", "FindAll", "FindAllIndex", "FindAllSubmatch":
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// regexp_quoteMeta is a thin wrapper around regexp.QuoteMeta used from handleRegexpCall.
// It lives here to avoid a package-level import of "regexp" that would add a large import.
func regexp_quoteMeta(s string) string {
	const special = `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// handleMathCall models math.* functions (#72).
// For concrete float64 arguments the real math function is called directly.
// For opaque arguments a conservative (non-NaN, non-Inf) sentinel is returned.
func (interp *Interpreter) handleMathCall(name string, args []Value) (Value, bool) {
	x, xok := stdlibArgFloat(args, 0)
	y, yok := stdlibArgFloat(args, 1)

	switch name {
	case "Abs":
		if xok {
			return Value{Raw: math.Abs(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Floor":
		if xok {
			return Value{Raw: math.Floor(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Ceil":
		if xok {
			return Value{Raw: math.Ceil(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Round":
		if xok {
			return Value{Raw: math.Round(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Trunc":
		if xok {
			return Value{Raw: math.Trunc(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Sqrt":
		if xok {
			return Value{Raw: math.Sqrt(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Cbrt":
		if xok {
			return Value{Raw: math.Cbrt(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Pow":
		if xok && yok {
			return Value{Raw: math.Pow(x, y)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Pow10":
		n, nok := stdlibArgInt(args, 0)
		if nok {
			return Value{Raw: math.Pow10(int(n))}, true
		}
		return Value{Raw: float64(1)}, true

	case "Log":
		if xok {
			return Value{Raw: math.Log(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Log2":
		if xok {
			return Value{Raw: math.Log2(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Log10":
		if xok {
			return Value{Raw: math.Log10(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Log1p":
		if xok {
			return Value{Raw: math.Log1p(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Exp":
		if xok {
			return Value{Raw: math.Exp(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Exp2":
		if xok {
			return Value{Raw: math.Exp2(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Expm1":
		if xok {
			return Value{Raw: math.Expm1(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Sin":
		if xok {
			return Value{Raw: math.Sin(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Cos":
		if xok {
			return Value{Raw: math.Cos(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Tan":
		if xok {
			return Value{Raw: math.Tan(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Asin":
		if xok {
			return Value{Raw: math.Asin(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Acos":
		if xok {
			return Value{Raw: math.Acos(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Atan":
		if xok {
			return Value{Raw: math.Atan(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Atan2":
		if xok && yok {
			return Value{Raw: math.Atan2(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Sinh", "Cosh", "Tanh":
		if xok {
			switch name {
			case "Sinh":
				return Value{Raw: math.Sinh(x)}, true
			case "Cosh":
				return Value{Raw: math.Cosh(x)}, true
			case "Tanh":
				return Value{Raw: math.Tanh(x)}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Hypot":
		if xok && yok {
			return Value{Raw: math.Hypot(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Min":
		if xok && yok {
			return Value{Raw: math.Min(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Max":
		if xok && yok {
			return Value{Raw: math.Max(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Mod":
		if xok && yok {
			return Value{Raw: math.Mod(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Remainder":
		if xok && yok {
			return Value{Raw: math.Remainder(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Dim":
		if xok && yok {
			return Value{Raw: math.Dim(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Inf":
		sign, sok := stdlibArgInt(args, 0)
		if sok {
			return Value{Raw: math.Inf(int(sign))}, true
		}
		return Value{Raw: math.Inf(1)}, true

	case "IsInf":
		if xok {
			sign := int64(0)
			if n, nok := stdlibArgInt(args, 1); nok {
				sign = n
			}
			return Value{Raw: math.IsInf(x, int(sign))}, true
		}
		return Value{Raw: false}, true

	case "IsNaN":
		if xok {
			return Value{Raw: math.IsNaN(x)}, true
		}
		return Value{Raw: false}, true

	case "NaN":
		return Value{Raw: math.NaN()}, true

	case "Signbit":
		if xok {
			return Value{Raw: math.Signbit(x)}, true
		}
		return Value{Raw: false}, true

	case "Copysign":
		if xok && yok {
			return Value{Raw: math.Copysign(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Logb":
		if xok {
			return Value{Raw: math.Logb(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Ilogb":
		if xok {
			return Value{Raw: int64(math.Ilogb(x))}, true
		}
		return Value{Raw: int64(0)}, true

	case "Frexp":
		if xok {
			frac, exp := math.Frexp(x)
			return Value{Raw: []Value{{Raw: frac}, {Raw: int64(exp)}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: int64(0)}}}, true

	case "Ldexp":
		if xok {
			exp, eok := stdlibArgInt(args, 1)
			if eok {
				return Value{Raw: math.Ldexp(x, int(exp))}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Modf":
		if xok {
			i, f := math.Modf(x)
			return Value{Raw: []Value{{Raw: float64(i)}, {Raw: f}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: float64(0)}}}, true

	case "J0", "J1":
		if xok {
			switch name {
			case "J0":
				return Value{Raw: math.J0(x)}, true
			case "J1":
				return Value{Raw: math.J1(x)}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Gamma":
		if xok {
			return Value{Raw: math.Gamma(x)}, true
		}
		return Value{Raw: float64(1)}, true

	case "Lgamma":
		if xok {
			lg, sign := math.Lgamma(x)
			return Value{Raw: []Value{{Raw: lg}, {Raw: int64(sign)}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: int64(1)}}}, true

	case "Erf", "Erfc":
		if xok {
			switch name {
			case "Erf":
				return Value{Raw: math.Erf(x)}, true
			case "Erfc":
				return Value{Raw: math.Erfc(x)}, true
			}
		}
		return Value{Raw: float64(0)}, true

	case "Sincos":
		if xok {
			s, c := math.Sincos(x)
			return Value{Raw: []Value{{Raw: s}, {Raw: c}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: float64(1)}}}, true

	case "Asinh":
		if xok {
			return Value{Raw: math.Asinh(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Acosh":
		if xok {
			return Value{Raw: math.Acosh(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Atanh":
		if xok {
			return Value{Raw: math.Atanh(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Float64bits":
		if xok {
			return Value{Raw: math.Float64bits(x)}, true
		}
		return Value{Raw: uint64(0)}, true

	case "Float64frombits":
		if b, bok := stdlibArgUint(args, 0); bok {
			return Value{Raw: math.Float64frombits(b)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Float32bits":
		if len(args) > 0 {
			switch f := args[0].Raw.(type) {
			case float32:
				return Value{Raw: math.Float32bits(f)}, true
			case float64:
				return Value{Raw: math.Float32bits(float32(f))}, true
			}
		}
		return Value{Raw: uint32(0)}, true

	case "Float32frombits":
		if len(args) > 0 {
			switch b := args[0].Raw.(type) {
			case uint32:
				return Value{Raw: math.Float32frombits(b)}, true
			case int64:
				return Value{Raw: math.Float32frombits(uint32(b))}, true
			case uint64:
				return Value{Raw: math.Float32frombits(uint32(b))}, true
			}
		}
		return Value{Raw: float32(0)}, true

	case "Nextafter":
		if xok && yok {
			return Value{Raw: math.Nextafter(x, y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Nextafter32":
		if len(args) >= 2 {
			if x32, ok := args[0].Raw.(float32); ok {
				if y32, ok2 := args[1].Raw.(float32); ok2 {
					return Value{Raw: math.Nextafter32(x32, y32)}, true
				}
			}
			// Fallback: interpret as float64 and convert.
			if xok && yok {
				return Value{Raw: math.Nextafter32(float32(x), float32(y))}, true
			}
		}
		return Value{Raw: float32(0)}, true

	case "Jn":
		n, nok := stdlibArgInt(args, 0)
		if nok && yok {
			return Value{Raw: math.Jn(int(n), y)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Y0":
		if xok {
			return Value{Raw: math.Y0(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Y1":
		if xok {
			return Value{Raw: math.Y1(x)}, true
		}
		return Value{Raw: float64(0)}, true

	case "Yn":
		n, nok := stdlibArgInt(args, 0)
		if nok && yok {
			return Value{Raw: math.Yn(int(n), y)}, true
		}
		return Value{Raw: float64(0)}, true
	}
	return Value{}, false
}

// handleUTF8Call models unicode/utf8.* functions (#75).
// For concrete string/rune arguments the real utf8 function is called; otherwise
// conservative non-zero values are returned to avoid blocking execution paths.
func (interp *Interpreter) handleUTF8Call(name string, args []Value) (Value, bool) {
	s0, s0ok := stdlibArgString(args, 0)
	r0, r0ok := stdlibArgInt(args, 0)

	switch name {
	case "RuneCountInString":
		if s0ok {
			return Value{Raw: int64(utf8.RuneCountInString(s0))}, true
		}
		return Value{Raw: int64(1)}, true

	case "RuneCount":
		// RuneCount(p []byte) int — conservative.
		return Value{Raw: int64(1)}, true

	case "ValidString":
		if s0ok {
			return Value{Raw: utf8.ValidString(s0)}, true
		}
		return Value{Raw: true}, true

	case "Valid":
		return Value{Raw: true}, true

	case "ValidRune":
		if r0ok {
			return Value{Raw: utf8.ValidRune(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "RuneLen":
		if r0ok {
			return Value{Raw: int64(utf8.RuneLen(rune(r0)))}, true
		}
		return Value{Raw: int64(1)}, true

	case "EncodeRune":
		// EncodeRune(p []byte, r rune) int — return byte size of the rune.
		r1, r1ok := stdlibArgInt(args, 1)
		if r1ok {
			return Value{Raw: int64(utf8.RuneLen(rune(r1)))}, true
		}
		return Value{Raw: int64(1)}, true

	case "DecodeRuneInString":
		// Returns (r rune, size int).
		if s0ok && len(s0) > 0 {
			r, size := utf8.DecodeRuneInString(s0)
			return Value{Raw: []Value{{Raw: int64(r)}, {Raw: int64(size)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "DecodeRune":
		// Returns (r rune, size int).
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "DecodeLastRuneInString":
		if s0ok && len(s0) > 0 {
			r, size := utf8.DecodeLastRuneInString(s0)
			return Value{Raw: []Value{{Raw: int64(r)}, {Raw: int64(size)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "DecodeLastRune":
		return Value{Raw: []Value{{Raw: int64('?')}, {Raw: int64(1)}}}, true

	case "FullRune", "FullRuneInString":
		return Value{Raw: true}, true

	case "AppendRune":
		// AppendRune(p []byte, r rune) []byte — conservative: return p unchanged.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{Raw: []byte{}}, true

	case "RuneError":
		return Value{Raw: int64(utf8.RuneError)}, true

	case "MaxRune":
		return Value{Raw: int64(utf8.MaxRune)}, true

	case "UTFMax":
		return Value{Raw: int64(utf8.UTFMax)}, true
	}
	// Suppress unused variable warnings for r0/r0ok used only in some branches.
	_ = r0
	_ = r0ok
	return Value{}, false
}

// handleUnicodeCall models unicode.* functions (#75).
// Predicates return true conservatively; transforms pass through.
func (interp *Interpreter) handleUnicodeCall(name string, args []Value) (Value, bool) {
	r0, r0ok := stdlibArgInt(args, 0)

	switch name {
	case "IsLetter":
		if r0ok {
			return Value{Raw: unicode.IsLetter(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsDigit":
		if r0ok {
			return Value{Raw: unicode.IsDigit(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsSpace":
		if r0ok {
			return Value{Raw: unicode.IsSpace(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsUpper":
		if r0ok {
			return Value{Raw: unicode.IsUpper(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsLower":
		if r0ok {
			return Value{Raw: unicode.IsLower(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsPunct":
		if r0ok {
			return Value{Raw: unicode.IsPunct(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsNumber":
		if r0ok {
			return Value{Raw: unicode.IsNumber(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsMark":
		if r0ok {
			return Value{Raw: unicode.IsMark(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsControl":
		if r0ok {
			return Value{Raw: unicode.IsControl(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "IsGraphic", "IsPrint":
		if r0ok {
			return Value{Raw: unicode.IsGraphic(rune(r0))}, true
		}
		return Value{Raw: true}, true

	case "IsTitle":
		if r0ok {
			return Value{Raw: unicode.IsTitle(rune(r0))}, true
		}
		return Value{Raw: false}, true

	case "ToLower":
		if r0ok {
			return Value{Raw: int64(unicode.ToLower(rune(r0)))}, true
		}
		return Value{Raw: r0}, true

	case "ToUpper":
		if r0ok {
			return Value{Raw: int64(unicode.ToUpper(rune(r0)))}, true
		}
		return Value{Raw: r0}, true

	case "ToTitle":
		if r0ok {
			return Value{Raw: int64(unicode.ToTitle(rune(r0)))}, true
		}
		return Value{Raw: r0}, true

	case "In":
		// unicode.In(r rune, ranges ...*RangeTable) bool — conservative.
		return Value{Raw: true}, true

	case "Is":
		// unicode.Is(rangeTab *RangeTable, r rune) bool — conservative.
		return Value{Raw: true}, true

	case "SimpleFold":
		if r0ok {
			return Value{Raw: int64(unicode.SimpleFold(rune(r0)))}, true
		}
		return Value{Raw: r0}, true
	}
	_ = r0
	_ = r0ok
	return Value{}, false
}

// handleContextCall models context.* functions (#76, #120).
// context.Background/TODO return an opaque non-nil value so downstream
// nil-checks on the context pass correctly.  WithCancel/WithTimeout/WithDeadline
// return a (ctx, cancelFunc) tuple where the cancel function is a cancelFuncID
// registered in interp.cancelFuncs. If the cancel function is never called,
// Finish() reports a ContextCancelLeakError.
func (interp *Interpreter) handleContextCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque

	switch name {
	case "Background", "TODO":
		return opaque, true

	case "WithCancel":
		// Returns (Context, CancelFunc). Register the cancel function for leak tracking.
		cfID := interp.newCancelFunc(gid, site)
		return Value{Raw: []Value{opaque, {Raw: cfID}}}, true

	case "WithTimeout", "WithDeadline":
		// Returns (Context, CancelFunc). Register the cancel function for leak tracking.
		cfID := interp.newCancelFunc(gid, site)
		return Value{Raw: []Value{opaque, {Raw: cfID}}}, true

	case "WithValue":
		// Returns Context (ignores key/value pair).
		return opaque, true

	case "WithCancelCause":
		// Go 1.20+: returns (Context, CancelCauseFunc). Register the cancel function.
		cfID := interp.newCancelFunc(gid, site)
		return Value{Raw: []Value{opaque, {Raw: cfID}}}, true

	case "Cause":
		// Returns nil error.
		return Value{}, true

	case "Done":
		// ctx.Done() returns a nil channel (never fires in our model).
		return Value{}, true

	case "Err":
		// ctx.Err() returns nil (no cancellation modeled).
		return Value{}, true

	case "Value":
		// ctx.Value(key) returns nil (no value propagation modeled).
		return Value{}, true

	case "Deadline":
		// ctx.Deadline() returns (zero time, false).
		return Value{Raw: []Value{{}, {Raw: false}}}, true
	}
	return Value{}, false
}

// handleAtomicCall models sync/atomic.* functions (#77).
// Atomic operations read and write through interp.valueStore keyed by the
// pointer argument's AllocID; they do NOT call handleLoad/handleStore to
// avoid false-positive race reports on atomic accesses.
func (interp *Interpreter) handleAtomicCall(name string, args []Value) (Value, bool) {
	var allocID shadow.AllocID
	if len(args) > 0 && args[0].Provenance != nil {
		allocID = args[0].Provenance.Alloc
	}

	switch name {
	case "LoadInt32", "LoadInt64", "LoadUint32", "LoadUint64", "LoadUintptr", "LoadPointer":
		if allocID != 0 && interp.valueStore != nil {
			if v, ok := interp.valueStore[allocID]; ok {
				return v, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "StoreInt32", "StoreInt64", "StoreUint32", "StoreUint64", "StoreUintptr", "StorePointer":
		if allocID != 0 && len(args) >= 2 && interp.valueStore != nil {
			interp.valueStore[allocID] = args[1]
		}
		return Value{}, true

	case "AddInt32", "AddInt64", "AddUint32", "AddUint64", "AddUintptr":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			newVal := Value{Raw: cur + toInt64(args[1])}
			interp.valueStore[allocID] = newVal
			return newVal, true
		}
		return Value{Raw: int64(0)}, true

	case "AndInt32", "AndInt64", "AndUint32", "AndUint64", "AndUintptr",
		"OrInt32", "OrInt64", "OrUint32", "OrUint64", "OrUintptr":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			delta := toInt64(args[1])
			var newRaw int64
			if name[:2] == "An" {
				newRaw = cur & delta
			} else {
				newRaw = cur | delta
			}
			newVal := Value{Raw: newRaw}
			interp.valueStore[allocID] = newVal
			return newVal, true
		}
		return Value{Raw: int64(0)}, true

	case "CompareAndSwapInt32", "CompareAndSwapInt64",
		"CompareAndSwapUint32", "CompareAndSwapUint64",
		"CompareAndSwapUintptr", "CompareAndSwapPointer":
		if allocID != 0 && len(args) >= 3 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			if cur == toInt64(args[1]) {
				interp.valueStore[allocID] = args[2]
				return Value{Raw: true}, true
			}
			return Value{Raw: false}, true
		}
		return Value{Raw: true}, true // pessimistic: assume CAS succeeds

	case "SwapInt32", "SwapInt64", "SwapUint32", "SwapUint64", "SwapUintptr", "SwapPointer":
		if allocID != 0 && len(args) >= 2 {
			old := Value{Raw: int64(0)}
			if v, ok := interp.valueStore[allocID]; ok {
				old = v
			}
			interp.valueStore[allocID] = args[1]
			return old, true
		}
		return Value{Raw: int64(0)}, true

	case "Value":
		// atomic.Value is a struct; Load/Store methods on it.
		// Method calls on atomic.Value have pkg path "sync/atomic".
		return Value{}, true

	// --- Added in v0.69.0: methods on Go 1.19+ concrete atomic types ---
	// atomic.Int32, Int64, Uint32, Uint64, Uintptr, Bool — concrete struct types
	// whose methods (Load, Store, Add, Swap, CompareAndSwap, And, Or) route here
	// as bare names because Function.Name() strips the receiver prefix.

	case "Load":
		if allocID != 0 && interp.valueStore != nil {
			if v, ok := interp.valueStore[allocID]; ok {
				return v, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "Store":
		if allocID != 0 && len(args) >= 2 && interp.valueStore != nil {
			interp.valueStore[allocID] = args[1]
		}
		return Value{}, true

	case "Add":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			newVal := Value{Raw: cur + toInt64(args[1])}
			interp.valueStore[allocID] = newVal
			return newVal, true
		}
		return Value{Raw: int64(0)}, true

	case "Swap":
		if allocID != 0 && len(args) >= 2 {
			old := Value{Raw: int64(0)}
			if v, ok := interp.valueStore[allocID]; ok {
				old = v
			}
			interp.valueStore[allocID] = args[1]
			return old, true
		}
		return Value{Raw: int64(0)}, true

	case "CompareAndSwap":
		if allocID != 0 && len(args) >= 3 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			if cur == toInt64(args[1]) {
				interp.valueStore[allocID] = args[2]
				return Value{Raw: true}, true
			}
			return Value{Raw: false}, true
		}
		return Value{Raw: true}, true // pessimistic: assume CAS succeeds

	case "And":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			newVal := Value{Raw: cur & toInt64(args[1])}
			interp.valueStore[allocID] = newVal
			return newVal, true
		}
		return Value{Raw: int64(0)}, true

	case "Or":
		if allocID != 0 && len(args) >= 2 {
			cur := int64(0)
			if v, ok := interp.valueStore[allocID]; ok {
				cur = toInt64(v)
			}
			newVal := Value{Raw: cur | toInt64(args[1])}
			interp.valueStore[allocID] = newVal
			return newVal, true
		}
		return Value{Raw: int64(0)}, true
	}
	return Value{}, false
}

// handleIOCall models io.* functions (#78).
func (interp *Interpreter) handleIOCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ReadAll":
		// io.ReadAll(r Reader) ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte("data")}, {}}}, true

	case "Copy", "CopyBuffer":
		// io.Copy(dst Writer, src Reader) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "CopyN":
		// io.CopyN(dst Writer, src Reader, n int64) (int64, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "WriteString":
		// io.WriteString(w Writer, s string) (n int, err error)
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Pipe":
		// io.Pipe() (*PipeReader, *PipeWriter)
		return Value{Raw: []Value{opaque, opaque}}, true

	case "NopCloser":
		// io.NopCloser(r Reader) ReadCloser
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "LimitReader":
		// io.LimitReader(r Reader, n int64) Reader
		return opaque, true

	case "MultiReader":
		// io.MultiReader(readers ...Reader) Reader
		return opaque, true

	case "MultiWriter":
		// io.MultiWriter(writers ...Writer) Writer
		return opaque, true

	case "TeeReader":
		// io.TeeReader(r Reader, w Writer) Reader
		return opaque, true

	case "NewSectionReader":
		// io.NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader
		return opaque, true

	case "ReadAtLeast", "ReadFull":
		// (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "Discard":
		// io.Discard is a variable (iota/Writer); return opaque.
		return opaque, true
	}
	return Value{}, false
}

// handleBufioCall models bufio.* functions (#78).
func (interp *Interpreter) handleBufioCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader", "NewReaderSize":
		return opaque, true
	case "NewWriter", "NewWriterSize":
		return opaque, true
	case "NewScanner":
		return opaque, true
	case "NewReadWriter":
		return opaque, true

	// Scanner methods (receiver = args[0]).
	case "Scan":
		// Always return false (scanner exhausted in our model).
		return Value{Raw: false}, true
	case "Text":
		return Value{Raw: ""}, true
	case "Bytes":
		return Value{Raw: []byte(nil)}, true
	case "Err":
		return Value{}, true
	case "Split":
		return Value{}, true
	case "Buffer":
		return Value{}, true

	// Reader/Writer flush methods.
	case "Flush":
		// (error)
		return Value{}, true
	case "Available":
		return Value{Raw: int64(0)}, true
	case "Buffered":
		return Value{Raw: int64(0)}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(0)}, true
	case "Discard":
		// (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	// bufio.Writer methods.
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "WriteString":
		n := 0
		if s, ok := stdlibArgString(args, 1); ok {
			n = len(s)
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "WriteByte":
		return Value{}, true
	case "WriteRune":
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	// bufio.Reader methods.
	case "ReadString":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "ReadBytes":
		// ReadBytes(delim byte) ([]byte, error) — return empty slice, nil error.
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "ReadLine":
		return Value{Raw: []Value{{Raw: []byte(nil)}, {Raw: false}, {}}}, true
	case "ReadByte":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "ReadRune":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {}}}, true
	case "ReadSlice":
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "Peek":
		return Value{Raw: []Value{{Raw: []byte(nil)}, {}}}, true
	case "UnreadByte":
		return Value{}, true
	case "UnreadRune":
		return Value{}, true
	}
	return Value{}, false
}

// handleLogCall models log.* functions (#80).
// log.Fatal/Fatalln/Fatalf mark all goroutines as Panicked (simulates os.Exit(1)).
// log.Panic/Panicln/Panicf mark the current goroutine as Panicked.
// log.Print/Println/Printf are noops (no output in the interpreter).
func (interp *Interpreter) handleLogCall(gid int64, name string, args []Value) (Value, bool) {
	switch name {
	case "Print", "Println", "Printf":
		return Value{}, true

	case "Fatal", "Fatalln", "Fatalf":
		// Simulates os.Exit(1): terminate all goroutines.
		for _, g := range interp.goroutines {
			g.Panicked = true
		}
		return Value{}, true

	case "Panic", "Panicln", "Panicf":
		// Mark only the current goroutine as panicked.
		if g := interp.goroutines[gid]; g != nil {
			g.Panicked = true
		}
		return Value{}, true

	case "New":
		// log.New(out, prefix, flags) *Logger — return opaque logger.
		return Value{Raw: struct{}{}}, true

	case "SetOutput", "SetFlags", "SetPrefix":
		return Value{}, true

	case "Flags":
		return Value{Raw: int64(0)}, true

	case "Prefix":
		return Value{Raw: ""}, true

	case "Writer":
		return Value{Raw: struct{}{}}, true

	case "Default":
		// log.Default() *Logger
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleHexCall models encoding/hex.* functions (#81).
func (interp *Interpreter) handleHexCall(name string, args []Value) (Value, bool) {
	switch name {
	case "EncodeToString":
		if len(args) > 0 {
			switch b := args[0].Raw.(type) {
			case []byte:
				return Value{Raw: hex.EncodeToString(b)}, true
			case []Value:
				bs := make([]byte, len(b))
				for i, v := range b {
					bs[i] = byte(toInt64(v))
				}
				return Value{Raw: hex.EncodeToString(bs)}, true
			}
		}
		return Value{Raw: "deadbeef"}, true // sentinel

	case "DecodeString":
		if s, ok := stdlibArgString(args, 0); ok {
			b, err := hex.DecodeString(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: []byte(nil)}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: b}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: []byte{0xde, 0xad}}, {}}}, true

	case "Encode":
		// hex.Encode(dst, src []byte) int
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				return Value{Raw: int64(hex.EncodedLen(len(b)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "Decode":
		// hex.Decode(dst, src []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "EncodedLen":
		if n, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: int64(hex.EncodedLen(int(n)))}, true
		}
		return Value{Raw: int64(0)}, true

	case "DecodedLen":
		if n, ok := stdlibArgInt(args, 0); ok {
			return Value{Raw: int64(hex.DecodedLen(int(n)))}, true
		}
		return Value{Raw: int64(0)}, true

	case "NewEncoder":
		return Value{Raw: struct{}{}}, true

	case "NewDecoder":
		return Value{Raw: struct{}{}}, true

	case "Dump":
		// Returns formatted hex dump string.
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleBase64Call models encoding/base64.* functions (#81).
func (interp *Interpreter) handleBase64Call(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}} // opaque *Encoding or io.ReadCloser

	switch name {
	case "StdEncoding", "URLEncoding", "RawStdEncoding", "RawURLEncoding":
		// These are package-level variables; return opaque *Encoding.
		return opaque, true

	case "NewEncoding":
		return opaque, true

	case "EncodeToString":
		// Called as method: enc.EncodeToString(src []byte) string
		// args[0] = receiver (*Encoding), args[1] = src
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				return Value{Raw: base64.StdEncoding.EncodeToString(b)}, true
			}
		}
		return Value{Raw: "aGVsbG8="}, true // base64("hello")

	case "DecodeString":
		// enc.DecodeString(s string) ([]byte, error)
		if len(args) >= 2 {
			if s, ok := stdlibArgString(args, 1); ok {
				b, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					// Try URL encoding.
					b, err = base64.URLEncoding.DecodeString(s)
				}
				if err == nil {
					return Value{Raw: []Value{{Raw: b}, {}}}, true
				}
				return Value{Raw: []Value{{Raw: []byte(nil)}, {Raw: err.Error()}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: []byte("data")}, {}}}, true

	case "Encode":
		// enc.Encode(dst, src []byte)
		return Value{}, true

	case "Decode":
		// enc.Decode(dst, src []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	case "EncodedLen":
		if len(args) >= 2 {
			if n, ok := stdlibArgInt(args, 1); ok {
				return Value{Raw: int64(base64.StdEncoding.EncodedLen(int(n)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "DecodedLen":
		if len(args) >= 2 {
			if n, ok := stdlibArgInt(args, 1); ok {
				return Value{Raw: int64(base64.StdEncoding.DecodedLen(int(n)))}, true
			}
		}
		return Value{Raw: int64(0)}, true

	case "NewEncoder":
		// NewEncoder(enc *Encoding, w io.Writer) io.WriteCloser
		return opaque, true

	case "NewDecoder":
		// NewDecoder(enc *Encoding, r io.Reader) io.Reader
		return opaque, true
	}
	return Value{}, false
}

// handleCryptoRandCall models crypto/rand.* functions (#82).
func (interp *Interpreter) handleCryptoRandCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Read":
		// rand.Read(b []byte) (int, error) — fills b with random bytes.
		n := 0
		if len(args) > 0 {
			switch b := args[0].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Int":
		// rand.Int(rand io.Reader, max *big.Int) (*big.Int, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Prime":
		// rand.Prime(rand io.Reader, bits int) (*big.Int, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Reader":
		// crypto/rand.Reader is a global; return opaque.
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleHashCall models crypto/md5, crypto/sha1, crypto/sha256, crypto/sha512 (#82).
// All four packages share the same API (hash.Hash interface), differing only in digest size.
func (interp *Interpreter) handleHashCall(pkgPath, name string, args []Value) (Value, bool) {
	// Digest lengths by package.
	digestLen := 16 // md5
	switch pkgPath {
	case "crypto/sha1":
		digestLen = 20
	case "crypto/sha256":
		digestLen = 32
	case "crypto/sha512":
		digestLen = 64
	}

	switch name {
	case "New", "New224", "New384":
		// Returns a hash.Hash (opaque interface value).
		return Value{Raw: struct{}{}}, true

	case "Sum":
		// Package-level sum function: md5.Sum(data []byte) [16]byte
		// Returns a fixed-size array; model as []byte sentinel.
		digest := make([]byte, digestLen)
		return Value{Raw: digest}, true

	case "Write":
		// h.Write(p []byte) (int, error)
		n := 0
		if len(args) > 1 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Sum32", "Sum64":
		// Sum function variants returning uint32/uint64.
		return Value{Raw: int64(0)}, true

	case "Reset":
		return Value{}, true

	case "Size":
		return Value{Raw: int64(digestLen)}, true

	case "BlockSize":
		// All common hash functions use 64-byte blocks (sha512 uses 128).
		if pkgPath == "crypto/sha512" {
			return Value{Raw: int64(128)}, true
		}
		return Value{Raw: int64(64)}, true
	}
	return Value{}, false
}

// handleFilepathCall models path/filepath.* functions (#83).
func (interp *Interpreter) handleFilepathCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Join":
		var parts []string
		for _, arg := range args {
			if s, ok := arg.Raw.(string); ok {
				parts = append(parts, s)
			} else {
				parts = append(parts, "path")
			}
		}
		return Value{Raw: filepath.Join(parts...)}, true

	case "Dir":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Dir(s)}, true
		}
		return Value{Raw: "/path"}, true

	case "Base":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Base(s)}, true
		}
		return Value{Raw: "file"}, true

	case "Ext":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Ext(s)}, true
		}
		return Value{Raw: ".txt"}, true

	case "Abs":
		if s, ok := stdlibArgString(args, 0); ok {
			a, err := filepath.Abs(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: s}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: a}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "/tmp/path"}, {}}}, true

	case "Clean":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.Clean(s)}, true
		}
		return Value{Raw: "."}, true

	case "IsAbs":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: filepath.IsAbs(s)}, true
		}
		return Value{Raw: false}, true

	case "Split":
		if s, ok := stdlibArgString(args, 0); ok {
			dir, file := filepath.Split(s)
			return Value{Raw: []Value{{Raw: dir}, {Raw: file}}}, true
		}
		return Value{Raw: []Value{{Raw: "/"}, {Raw: "file"}}}, true

	case "Rel":
		if s0, ok0 := stdlibArgString(args, 0); ok0 {
			if s1, ok1 := stdlibArgString(args, 1); ok1 {
				rel, err := filepath.Rel(s0, s1)
				if err != nil {
					return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
				}
				return Value{Raw: []Value{{Raw: rel}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: "rel/path"}, {}}}, true

	case "Match":
		// filepath.Match(pattern, name string) (matched bool, err error)
		if s0, ok0 := stdlibArgString(args, 0); ok0 {
			if s1, ok1 := stdlibArgString(args, 1); ok1 {
				matched, err := filepath.Match(s0, s1)
				if err != nil {
					return Value{Raw: []Value{{Raw: false}, {Raw: err.Error()}}}, true
				}
				return Value{Raw: []Value{{Raw: matched}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: true}, {}}}, true // pessimistic

	case "Glob":
		// filepath.Glob(pattern string) (matches []string, err error)
		return Value{Raw: []Value{{Raw: []Value{{Raw: "file.txt"}}}, {}}}, true

	case "Walk", "WalkDir":
		// Noop — no filesystem access in the interpreter.
		return Value{}, true

	case "FromSlash", "ToSlash":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: "path"}, true

	case "VolumeName":
		return Value{Raw: ""}, true

	case "EvalSymlinks":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: []Value{{Raw: s}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "/path"}, {}}}, true

	case "SplitList":
		if s, ok := stdlibArgString(args, 0); ok {
			parts := filepath.SplitList(s)
			vals := make([]Value, len(parts))
			for i, p := range parts {
				vals[i] = Value{Raw: p}
			}
			return Value{Raw: vals}, true
		}
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handlePathCall models path.* functions (non-OS, slash-only paths) (#83).
func (interp *Interpreter) handlePathCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Join":
		var parts []string
		for _, arg := range args {
			if s, ok := arg.Raw.(string); ok {
				parts = append(parts, s)
			} else {
				parts = append(parts, "seg")
			}
		}
		// Use filepath.Join then replace OS separator with slash.
		result := strings.ReplaceAll(filepath.Join(parts...), string(filepath.Separator), "/")
		return Value{Raw: result}, true

	case "Dir":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, "/")
			if idx < 0 {
				return Value{Raw: "."}, true
			}
			return Value{Raw: s[:idx]}, true
		}
		return Value{Raw: "."}, true

	case "Base":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, "/")
			if idx < 0 {
				return Value{Raw: s}, true
			}
			return Value{Raw: s[idx+1:]}, true
		}
		return Value{Raw: "file"}, true

	case "Ext":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, ".")
			if idx < 0 {
				return Value{Raw: ""}, true
			}
			return Value{Raw: s[idx:]}, true
		}
		return Value{Raw: ""}, true

	case "Clean":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strings.TrimRight(s, "/")}, true
		}
		return Value{Raw: "."}, true

	case "IsAbs":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: strings.HasPrefix(s, "/")}, true
		}
		return Value{Raw: false}, true

	case "Split":
		if s, ok := stdlibArgString(args, 0); ok {
			idx := strings.LastIndex(s, "/")
			if idx < 0 {
				return Value{Raw: []Value{{Raw: ""}, {Raw: s}}}, true
			}
			return Value{Raw: []Value{{Raw: s[:idx+1]}, {Raw: s[idx+1:]}}}, true
		}
		return Value{Raw: []Value{{Raw: "/"}, {Raw: "file"}}}, true

	case "Match":
		return Value{Raw: []Value{{Raw: true}, {}}}, true // pessimistic
	}
	return Value{}, false
}

// handleNetCall models net.* utility functions (#84).
// Full socket/connection model is not implemented; only pure utility functions.
func (interp *Interpreter) handleNetCall(name string, args []Value) (Value, bool) {
	switch name {
	case "SplitHostPort":
		if s, ok := stdlibArgString(args, 0); ok {
			host, port, err := net.SplitHostPort(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: host}, {Raw: port}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "localhost"}, {Raw: "8080"}, {}}}, true

	case "JoinHostPort":
		host, hostOK := stdlibArgString(args, 0)
		port, portOK := stdlibArgString(args, 1)
		if hostOK && portOK {
			return Value{Raw: net.JoinHostPort(host, port)}, true
		}
		return Value{Raw: "localhost:8080"}, true

	case "ParseIP":
		if s, ok := stdlibArgString(args, 0); ok {
			ip := net.ParseIP(s)
			if ip == nil {
				return Value{}, true // nil IP
			}
			return Value{Raw: ip.String()}, true
		}
		return Value{Raw: "127.0.0.1"}, true // sentinel

	case "ParseCIDR":
		if s, ok := stdlibArgString(args, 0); ok {
			ip, ipnet, err := net.ParseCIDR(s)
			if err != nil {
				return Value{Raw: []Value{{}, {}, {Raw: err.Error()}}}, true
			}
			_ = ipnet
			return Value{Raw: []Value{{Raw: ip.String()}, {Raw: struct{}{}}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "127.0.0.1"}, {Raw: struct{}{}}, {}}}, true

	case "LookupHost":
		// Conservative: do not actually resolve; return sentinel.
		return Value{Raw: []Value{{Raw: []Value{{Raw: "127.0.0.1"}}}, {}}}, true

	case "LookupPort":
		s, _ := stdlibArgString(args, 1)
		port := int64(8080)
		switch s {
		case "http":
			port = 80
		case "https":
			port = 443
		case "ftp":
			port = 21
		case "ssh":
			port = 22
		}
		return Value{Raw: []Value{{Raw: port}, {}}}, true

	case "LookupIP", "LookupTXT", "LookupMX", "LookupNS", "LookupCNAME":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true

	case "ResolveTCPAddr", "ResolveUDPAddr", "ResolveIPAddr", "ResolveUnixAddr":
		// (network, addr string) → (*Addr, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Dial", "DialTimeout":
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Listen", "ListenPacket":
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Pipe":
		opaque := stdlibOpaque
		return Value{Raw: []Value{opaque, opaque}}, true

	case "IPv4", "IPv4Mask":
		return Value{Raw: "0.0.0.0"}, true

	case "CIDRMask":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleTemplateCall models text/template and html/template functions (#84).
// Both packages share the same API; html/template escapes output but the
// interpreter models both identically.
func (interp *Interpreter) handleTemplateCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		// template.New(name string) *Template
		return opaque, true

	case "Must":
		// template.Must(t *Template, err error) *Template
		// If err is nil return t, otherwise return opaque.
		if len(args) >= 2 && args[1].Raw == nil {
			return args[0], true
		}
		return opaque, true

	case "ParseFiles", "ParseGlob":
		// template.ParseFiles/ParseGlob(...) (*Template, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "Parse":
		// t.Parse(text string) (*Template, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "ParseFS":
		return Value{Raw: []Value{opaque, {}}}, true

	case "Execute":
		// t.Execute(wr io.Writer, data interface{}) error — return nil.
		return Value{}, true

	case "ExecuteTemplate":
		// t.ExecuteTemplate(wr io.Writer, name string, data interface{}) error
		return Value{}, true

	case "Funcs":
		// t.Funcs(funcMap FuncMap) *Template — chainable.
		return opaque, true

	case "Delims":
		return opaque, true

	case "Lookup":
		// t.Lookup(name string) *Template
		return opaque, true

	case "Name":
		// t.Name() string
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: "tmpl"}, true

	case "Clone":
		return Value{Raw: []Value{opaque, {}}}, true

	case "Templates":
		// t.Templates() []*Template
		return Value{Raw: []Value{opaque}}, true

	case "Option":
		return opaque, true

	case "HTMLEscape", "HTMLEscapeString", "JSEscape", "JSEscapeString",
		"URLQueryEscaper":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true // pass-through for non-dangerous input
		}
		return Value{Raw: "escaped"}, true
	}
	return Value{}, false
}

// handleXMLCall models encoding/xml.* functions (#87).
func (interp *Interpreter) handleXMLCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Marshal":
		// xml.Marshal(v interface{}) ([]byte, error)
		if len(args) > 0 && args[0].Raw != nil {
			b, err := xml.Marshal(args[0].Raw)
			if err == nil {
				return Value{Raw: []Value{{Raw: b}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: []byte("<sentinel/>")}, {}}}, true

	case "MarshalIndent":
		return Value{Raw: []Value{{Raw: []byte("<sentinel/>")}, {}}}, true

	case "Unmarshal":
		// xml.Unmarshal(data []byte, v interface{}) error — return nil error.
		return Value{}, true

	case "NewDecoder":
		return opaque, true

	case "NewEncoder":
		return opaque, true

	case "NewTokenDecoder":
		return opaque, true

	case "Decode":
		// d.Decode(v interface{}) error
		return Value{}, true

	case "DecodeElement":
		return Value{}, true

	case "Token":
		// d.Token() (Token, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "Encode":
		// e.Encode(v interface{}) error
		return Value{}, true

	case "EncodeElement":
		return Value{}, true

	case "EncodeToken":
		return Value{}, true

	case "Flush":
		return Value{}, true

	case "EscapeText":
		return Value{}, true

	case "Escape":
		return Value{}, true

	case "CopyToken":
		return opaque, true

	case "Name", "Attr", "CharData", "Comment", "ProcInst", "Directive",
		"StartElement", "EndElement":
		// XML token types — return opaque.
		return opaque, true
	}
	return Value{}, false
}

// handleCSVCall models encoding/csv.* functions (#87).
func (interp *Interpreter) handleCSVCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader":
		return opaque, true

	case "NewWriter":
		return opaque, true

	case "Read":
		// r.Read() (record []string, err error)
		return Value{Raw: []Value{{Raw: []Value{{Raw: "field1"}, {Raw: "field2"}}}, {}}}, true

	case "ReadAll":
		// r.ReadAll() (records [][]string, err error)
		row := []Value{{Raw: "f1"}, {Raw: "f2"}}
		records := []Value{{Raw: row}}
		return Value{Raw: []Value{{Raw: records}, {}}}, true

	case "Write":
		// w.Write(record []string) error
		return Value{}, true

	case "WriteAll":
		// w.WriteAll(records [][]string) error
		return Value{}, true

	case "Flush":
		return Value{}, true

	case "Error":
		// w.Error() error — return nil
		return Value{}, true
	}
	_ = opaque
	return Value{}, false
}

// handleReflectCall models reflect.* functions (#86).
// Only non-unsafe reflect functions are handled here; Pointer and UnsafeAddr
// are intercepted earlier in exec.go for Rule 5 checking.
func (interp *Interpreter) handleReflectCall(name string, args []Value) (Value, bool) {
	opaque := Value{Raw: struct{}{}} // sentinel for reflect.Type / reflect.Value
	switch name {
	case "TypeOf":
		// reflect.TypeOf(v interface{}) reflect.Type — return opaque non-nil Type.
		return opaque, true

	case "ValueOf":
		// reflect.ValueOf(v interface{}) reflect.Value — return the value as-is.
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "DeepEqual":
		// reflect.DeepEqual(x, y interface{}) bool
		if len(args) >= 2 && args[0].Raw != nil && args[1].Raw != nil {
			return Value{Raw: reflect.DeepEqual(args[0].Raw, args[1].Raw)}, true
		}
		return Value{Raw: true}, true // pessimistic: assume equal

	case "New":
		// reflect.New(t reflect.Type) reflect.Value — return opaque pointer.
		return opaque, true

	case "Zero":
		// reflect.Zero(t reflect.Type) reflect.Value
		return opaque, true

	case "MakeSlice":
		// reflect.MakeSlice(t, len, cap int) reflect.Value
		return opaque, true

	case "MakeMap", "MakeMapWithSize":
		return opaque, true

	case "MakeChan":
		return opaque, true

	case "MakeFunc":
		return opaque, true

	case "Append", "AppendSlice":
		return opaque, true

	case "Copy":
		// reflect.Copy(dst, src Value) int
		return Value{Raw: int64(0)}, true

	case "Indirect":
		// reflect.Indirect(v Value) Value
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "PtrTo", "PointerTo":
		return opaque, true

	case "SliceOf":
		return opaque, true

	case "ArrayOf":
		return opaque, true

	case "MapOf":
		return opaque, true

	case "ChanOf":
		return opaque, true

	case "FuncOf":
		return opaque, true

	case "StructOf":
		return opaque, true

	// reflect.Value method calls (receiver = args[0]):
	case "Kind":
		// v.Kind() reflect.Kind — return sentinel (Struct=25)
		return Value{Raw: int64(25)}, true

	case "Type":
		return opaque, true

	case "Interface":
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "Elem":
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true

	case "Field":
		return opaque, true

	case "Index":
		return opaque, true

	case "MapIndex":
		return opaque, true

	case "MapKeys":
		return Value{Raw: []Value{}}, true

	case "NumField":
		return Value{Raw: int64(0)}, true

	case "NumMethod":
		return Value{Raw: int64(0)}, true

	case "Method", "MethodByName":
		return opaque, true

	case "Len", "Cap":
		return Value{Raw: int64(0)}, true

	case "IsNil":
		return Value{Raw: false}, true // pessimistic: assume not nil

	case "IsValid":
		return Value{Raw: true}, true // pessimistic: assume valid

	case "IsZero":
		return Value{Raw: false}, true

	case "CanAddr", "CanSet", "CanInterface":
		return Value{Raw: true}, true

	case "Set", "SetInt", "SetUint", "SetFloat", "SetBool", "SetString",
		"SetBytes", "SetCap", "SetLen", "SetPointer", "SetIterKey", "SetIterValue":
		return Value{}, true

	case "Int":
		return Value{Raw: int64(0)}, true

	case "Uint":
		return Value{Raw: int64(0)}, true

	case "Float":
		return Value{Raw: float64(0)}, true

	case "Bool":
		return Value{Raw: false}, true

	case "String":
		return Value{Raw: ""}, true

	case "Bytes":
		return Value{Raw: []byte(nil)}, true

	case "Addr":
		return opaque, true

	case "Call", "CallSlice":
		// v.Call(in []Value) []Value — return empty slice (no actual dispatch).
		return Value{Raw: []Value{}}, true

	case "Convert":
		return opaque, true

	case "Recv":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	case "Send":
		return Value{}, true

	case "Close":
		return Value{}, true

	case "TrySend", "TryRecv":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	// reflect.Type method calls:
	case "Name":
		return Value{Raw: "T"}, true

	case "PkgPath":
		return Value{Raw: ""}, true

	case "Size":
		return Value{Raw: int64(8)}, true

	case "Implements":
		return Value{Raw: true}, true // pessimistic

	case "AssignableTo", "ConvertibleTo", "Comparable":
		return Value{Raw: true}, true

	case "In":
		return opaque, true

	case "Out":
		return opaque, true

	case "NumIn", "NumOut":
		return Value{Raw: int64(0)}, true

	case "Key":
		return opaque, true

	case "ChanDir":
		return Value{Raw: int64(0)}, true

	case "IsVariadic":
		return Value{Raw: false}, true

	case "Bits":
		return Value{Raw: int64(64)}, true

	case "FieldByName", "FieldByIndex", "FieldByNameFunc":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	case "MethodByName2":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true

	case "Align", "FieldAlign":
		return Value{Raw: int64(8)}, true
	}
	return Value{}, false
}

// handleFlagCall models flag.* functions (#88).
// Flag-defined values return opaque non-nil pointers so nil-checks on them pass.
// Parse is a noop; command-line arguments cannot be modeled at analysis time.
func (interp *Interpreter) handleFlagCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Flag definition functions — return a non-nil pointer to the flag value.
	case "String", "StringVar":
		if name == "StringVar" {
			return Value{}, true // sets *string in place, no return
		}
		p := new(string)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(string); ok {
				*p = def
			}
		}
		return Value{Raw: p}, true
	case "Int", "IntVar":
		if name == "IntVar" {
			return Value{}, true
		}
		p := new(int)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(int64); ok {
				*p = int(def)
			}
		}
		return Value{Raw: p}, true
	case "Int64", "Int64Var":
		if name == "Int64Var" {
			return Value{}, true
		}
		p := new(int64)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(int64); ok {
				*p = def
			}
		}
		return Value{Raw: p}, true
	case "Uint", "UintVar":
		if name == "UintVar" {
			return Value{}, true
		}
		p := new(uint)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(int64); ok {
				*p = uint(def)
			}
		}
		return Value{Raw: p}, true
	case "Uint64", "Uint64Var":
		if name == "Uint64Var" {
			return Value{}, true
		}
		p := new(uint64)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(int64); ok {
				*p = uint64(def)
			}
		}
		return Value{Raw: p}, true
	case "Bool", "BoolVar":
		if name == "BoolVar" {
			return Value{}, true
		}
		p := new(bool)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(bool); ok {
				*p = def
			}
		}
		return Value{Raw: p}, true
	case "Float64", "Float64Var":
		if name == "Float64Var" {
			return Value{}, true
		}
		p := new(float64)
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(float64); ok {
				*p = def
			}
		}
		return Value{Raw: p}, true
	case "Duration", "DurationVar":
		if name == "DurationVar" {
			return Value{}, true
		}
		p := new(int64) // time.Duration is int64
		if len(args) >= 2 {
			if def, ok := args[1].Raw.(int64); ok {
				*p = def
			}
		}
		return Value{Raw: p}, true

	case "Func":
		// flag.Func(name, usage string, fn func(string) error)
		return Value{}, true

	case "TextVar":
		return Value{}, true

	// Parsing
	case "Parse":
		return Value{}, true

	case "Parsed":
		return Value{Raw: true}, true // assume parsed

	// Introspection
	case "Arg":
		return Value{Raw: ""}, true

	case "Args":
		return Value{Raw: []Value{}}, true

	case "NArg", "NFlag":
		return Value{Raw: int64(0)}, true

	case "Lookup":
		// Returns *flag.Flag (nil if not found — conservative).
		return Value{}, true

	case "Set":
		// flag.Set(name, value string) error
		return Value{}, true

	case "Visit", "VisitAll":
		return Value{}, true

	case "PrintDefaults":
		return Value{}, true

	case "Usage":
		return Value{}, true

	case "CommandLine":
		return opaque, true

	case "NewFlagSet":
		return opaque, true

	case "UnquoteUsage":
		return Value{Raw: []Value{{Raw: ""}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleRuntimeCall models runtime.* functions (#88).
// Most runtime functions are noops or return sentinel integers.
func (interp *Interpreter) handleRuntimeCall(name string, args []Value) (Value, bool) {
	switch name {
	case "NumCPU":
		return Value{Raw: int64(runtime.NumCPU())}, true

	case "GOMAXPROCS":
		prev := runtime.GOMAXPROCS(0) // query without changing
		if len(args) > 0 {
			if n, ok := stdlibArgInt(args, 0); ok && n > 0 {
				runtime.GOMAXPROCS(int(n))
			}
		}
		return Value{Raw: int64(prev)}, true

	case "NumGoroutine":
		return Value{Raw: int64(1)}, true // model: 1 (our goroutines are virtual)

	case "NumCgoCall":
		return Value{Raw: int64(0)}, true

	case "Caller":
		// runtime.Caller(skip int) (pc uintptr, file string, line int, ok bool)
		return Value{Raw: []Value{
			{Raw: int64(0)},
			{Raw: "giri.go"},
			{Raw: int64(1)},
			{Raw: false}, // conservative: stack not available
		}}, true

	case "Callers":
		// runtime.Callers(skip int, pc []uintptr) int
		return Value{Raw: int64(0)}, true

	case "FuncForPC":
		// Returns *runtime.Func (nil — no debug info available).
		return Value{}, true

	case "CallersFrames":
		// Returns *runtime.Frames (opaque).
		return Value{Raw: struct{}{}}, true

	case "GC":
		return Value{}, true

	case "Gosched":
		return Value{}, true

	case "LockOSThread", "UnlockOSThread":
		return Value{}, true

	case "Goexit":
		// Terminates the current goroutine — noop in the interpreter
		// (the goroutine naturally stops returning from execFunction).
		return Value{}, true

	case "Stack":
		// runtime.Stack(buf []byte, all bool) int
		return Value{Raw: int64(0)}, true

	case "Version":
		return Value{Raw: runtime.Version()}, true

	case "ReadMemStats":
		return Value{}, true

	case "SetFinalizer":
		return Value{}, true

	case "KeepAlive":
		return Value{}, true

	case "SetBlockProfileRate", "SetMutexProfileFraction", "SetCPUProfileRate":
		return Value{}, true

	case "Breakpoint":
		return Value{}, true

	case "SetPanicOnFault":
		return Value{Raw: false}, true

	case "GOARCH":
		return Value{Raw: runtime.GOARCH}, true

	case "GOOS":
		return Value{Raw: runtime.GOOS}, true

	case "GOROOT":
		// runtime.GOROOT is deprecated since Go 1.24; use environment variable instead.
		goroot := os.Getenv("GOROOT")
		return Value{Raw: goroot}, true
	}
	return Value{}, false
}

// handleNetURLCall models net/url.* functions (#89).
func (interp *Interpreter) handleNetURLCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque

	switch name {
	case "Parse", "ParseRequestURI":
		// url.Parse(rawurl string) (*URL, error)
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := url.Parse(s)
			if err != nil {
				return Value{Raw: []Value{{}, {Raw: err.Error()}}}, true
			}
			// Return the URL as an opaque value; downstream field accesses
			// (Scheme, Host, Path, etc.) are intercepted as method calls.
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{opaque, {}}}, true

	case "ParseQuery":
		// url.ParseQuery(query string) (Values, error)
		if s, ok := stdlibArgString(args, 0); ok {
			vals, err := url.ParseQuery(s)
			if err != nil {
				return Value{Raw: []Value{{}, {Raw: err.Error()}}}, true
			}
			// Model Values as opaque; downstream Get/Set calls are intercepted.
			return Value{Raw: []Value{{Raw: vals}, {}}}, true
		}
		return Value{Raw: []Value{opaque, {}}}, true

	case "QueryEscape":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: url.QueryEscape(s)}, true
		}
		return Value{Raw: "escaped"}, true

	case "QueryUnescape":
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := url.QueryUnescape(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "unescaped"}, {}}}, true

	case "PathEscape":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: url.PathEscape(s)}, true
		}
		return Value{Raw: "escaped"}, true

	case "PathUnescape":
		if s, ok := stdlibArgString(args, 0); ok {
			u, err := url.PathUnescape(s)
			if err != nil {
				return Value{Raw: []Value{{Raw: ""}, {Raw: err.Error()}}}, true
			}
			return Value{Raw: []Value{{Raw: u}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "unescaped"}, {}}}, true

	case "User":
		// url.User(username string) *Userinfo
		return opaque, true

	case "UserPassword":
		// url.UserPassword(username, password string) *Userinfo
		return opaque, true

	case "JoinPath":
		// url.JoinPath(base string, elem ...string) (string, error)
		if s, ok := stdlibArgString(args, 0); ok {
			parts := []string{s}
			for _, a := range args[1:] {
				if p, ok2 := a.Raw.(string); ok2 {
					parts = append(parts, p)
				}
			}
			return Value{Raw: []Value{{Raw: strings.Join(parts, "/")}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "http://example.com/path"}, {}}}, true

	// *url.URL method calls (receiver = args[0]):
	case "String":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.String()}, true
			}
		}
		return Value{Raw: "http://example.com"}, true

	case "Scheme":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Scheme}, true
			}
		}
		return Value{Raw: "https"}, true

	case "Host":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Host}, true
			}
		}
		return Value{Raw: "example.com"}, true

	case "Path":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Path}, true
			}
		}
		return Value{Raw: "/path"}, true

	case "RawQuery":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.RawQuery}, true
			}
		}
		return Value{Raw: "key=value"}, true

	case "Fragment":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Fragment}, true
			}
		}
		return Value{Raw: ""}, true

	case "Query":
		// u.Query() url.Values
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Query()}, true
			}
		}
		return opaque, true

	case "Hostname":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Hostname()}, true
			}
		}
		return Value{Raw: "example.com"}, true

	case "Port":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.Port()}, true
			}
		}
		return Value{Raw: ""}, true

	case "RequestURI":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.RequestURI()}, true
			}
		}
		return Value{Raw: "/path?key=value"}, true

	case "ResolveReference":
		return opaque, true

	case "IsAbs":
		if len(args) > 0 {
			if u, ok := args[0].Raw.(*url.URL); ok {
				return Value{Raw: u.IsAbs()}, true
			}
		}
		return Value{Raw: true}, true

	case "MarshalBinary":
		return Value{Raw: []Value{{Raw: []byte("url")}, {}}}, true

	case "UnmarshalBinary":
		return Value{}, true

	case "EscapedPath":
		return Value{Raw: "/path"}, true

	case "EscapedFragment":
		return Value{Raw: ""}, true

	// url.Values method calls (receiver = args[0]):
	case "Get":
		if len(args) >= 2 {
			if vals, ok := args[0].Raw.(url.Values); ok {
				if k, ok2 := stdlibArgString(args, 1); ok2 {
					return Value{Raw: vals.Get(k)}, true
				}
			}
		}
		return Value{Raw: "value"}, true

	case "Set":
		if len(args) >= 3 {
			if vals, ok := args[0].Raw.(url.Values); ok {
				k, _ := stdlibArgString(args, 1)
				v, _ := stdlibArgString(args, 2)
				vals.Set(k, v)
			}
		}
		return Value{}, true

	case "Add":
		return Value{}, true

	case "Del":
		return Value{}, true

	case "Has":
		return Value{Raw: true}, true // pessimistic

	case "Encode":
		if len(args) > 0 {
			if vals, ok := args[0].Raw.(url.Values); ok {
				return Value{Raw: vals.Encode()}, true
			}
		}
		return Value{Raw: "key=value"}, true
	}
	return Value{}, false
}

// handleExecCall models os/exec.* functions (#90).
func (interp *Interpreter) handleExecCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Command", "CommandContext":
		// exec.Command(name string, arg ...string) *Cmd
		return opaque, true

	case "LookPath":
		// exec.LookPath(file string) (string, error)
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: []Value{{Raw: "/usr/bin/" + s}, {}}}, true
		}
		return Value{Raw: []Value{{Raw: "/usr/bin/cmd"}, {}}}, true

	// *exec.Cmd method calls (receiver = args[0]):
	case "Run":
		// cmd.Run() error — return nil (assume success)
		return Value{}, true

	case "Output":
		// cmd.Output() ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte("output")}, {}}}, true

	case "CombinedOutput":
		// cmd.CombinedOutput() ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte("output")}, {}}}, true

	case "Start":
		// cmd.Start() error
		return Value{}, true

	case "Wait":
		// cmd.Wait() error
		return Value{}, true

	case "StdoutPipe":
		// cmd.StdoutPipe() (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "StderrPipe":
		// cmd.StderrPipe() (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "StdinPipe":
		// cmd.StdinPipe() (io.WriteCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "String":
		// cmd.String() string
		return Value{Raw: "cmd"}, true

	case "Environ":
		// cmd.Environ() []string
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handleGzipCall models compress/gzip.* functions (#91).
func (interp *Interpreter) handleGzipCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader":
		// gzip.NewReader(r io.Reader) (*Reader, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewWriter":
		// gzip.NewWriter(w io.Writer) *Writer — single return value.
		return opaque, true

	case "NewWriterLevel":
		// gzip.NewWriterLevel(w io.Writer, level int) (*Writer, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "BestCompression", "BestSpeed", "DefaultCompression", "HuffmanOnly",
		"NoCompression":
		// gzip level constants — should not be called but handle gracefully.
		return Value{Raw: int64(-1)}, true

	// *gzip.Reader method calls:
	case "Read":
		// r.Read(p []byte) (int, error) — return EOF immediately.
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: "EOF"}}}, true

	case "Close":
		return Value{}, true

	case "Reset":
		return Value{}, true

	case "Multistream":
		return Value{}, true

	// *gzip.Writer method calls:
	case "Write":
		n := 0
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Flush":
		return Value{}, true
	}
	_ = opaque
	return Value{}, false
}

// handleHTTPCall models net/http client and server functions (#95).
// Server-side operations (ListenAndServe, Handle) are noops.
// Client calls (Get/Post/NewRequest/Do) return opaque (*Response, nil) pairs.
// Field accesses on *http.Response go through SSA FieldAddr on the opaque value
// and cannot be resolved; tests should avoid direct field reads.
func (interp *Interpreter) handleHTTPCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Package-level client functions:
	case "Get":
		// Disambiguate: package-level http.Get(url) has 1 arg;
		// (http.Header).Get(key) or (*http.Client).Get(url) have 2 args.
		// When the receiver (args[0]) is nil/zero (e.g. from FieldAddr on an opaque
		// *http.Request), treat as http.Header.Get and return a string.
		if len(args) == 1 {
			return Value{Raw: []Value{opaque, {}}}, true // http.Get(url) → (*Response, error)
		}
		if len(args) >= 2 && args[0].Raw == nil {
			return Value{Raw: "value"}, true // http.Header.Get(key) → string
		}
		return Value{Raw: []Value{opaque, {}}}, true // (*http.Client).Get(url) → (*Response, error)

	case "Post", "Head", "PostForm":
		// (*Response, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewRequest", "NewRequestWithContext":
		// (*Request, error)
		return Value{Raw: []Value{opaque, {}}}, true

	// *http.Client methods:
	case "Do":
		// (*Response, error)
		return Value{Raw: []Value{opaque, {}}}, true

	// Server-side helpers — noops:
	case "ListenAndServe", "ListenAndServeTLS":
		return Value{}, true
	case "Handle", "HandleFunc":
		return Value{}, true
	case "ServeHTTP", "ServeFile", "ServeContent":
		return Value{}, true
	case "Error", "Redirect", "NotFound", "NotFoundHandler":
		return Value{}, true
	case "StripPrefix":
		return opaque, true

	// Mux construction:
	case "NewServeMux":
		return opaque, true

	// *ServeMux methods:
	case "Handler", "ServeHTTP2":
		return opaque, true

	// Status text helper:
	case "StatusText":
		return Value{Raw: ""}, true

	// *http.Request methods:
	case "FormValue", "PostFormValue":
		return Value{Raw: ""}, true
	case "ParseForm", "ParseMultipartForm":
		return Value{}, true
	case "WithContext":
		return opaque, true
	case "Clone":
		return opaque, true
	case "Context":
		// (*http.Request).Context() context.Context
		return opaque, true

	// http.Header method calls (map[string][]string):
	case "Set", "Add", "Del":
		// Header.Set/Add/Del — mutate header, no return value.
		return Value{}, true
	case "Values":
		// Header.Values(key) []string — return pessimistic non-empty slice.
		return Value{Raw: []Value{{Raw: "value"}}}, true
	case "CanonicalHeaderKey":
		// http.CanonicalHeaderKey(s) string — return input unchanged if known.
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: "Content-Type"}, true

	// http.ResponseWriter method calls:
	case "WriteHeader":
		// ResponseWriter.WriteHeader(statusCode int) — noop.
		return Value{}, true

	// *http.Response methods (Body is io.ReadCloser — handled by io intercept):
	case "Cookies":
		return Value{Raw: []Value{}}, true

	// Transport / round-tripper:
	case "RoundTrip":
		return Value{Raw: []Value{opaque, {}}}, true

	// Cookie helpers:
	case "SetCookie":
		return Value{}, true
	case "ReadResponse":
		return Value{Raw: []Value{opaque, {}}}, true

	// Misc helpers:
	case "MaxBytesReader":
		return opaque, true
	case "DetectContentType":
		return Value{Raw: "application/octet-stream"}, true
	}
	return Value{}, false
}

// handleSignalCall models os/signal functions (#96).
// Notify pre-populates the target channel so goroutines waiting on it
// are not falsely flagged as leaked.
func (interp *Interpreter) handleSignalCall(gid int64, name string, args []Value) (Value, bool) {
	switch name {
	case "Notify":
		// signal.Notify(ch chan<- os.Signal, sig ...os.Signal)
		// Pre-populate the channel so it immediately has a pending value.
		if len(args) >= 1 {
			if chanID, ok := args[0].Raw.(ChanID); ok {
				if ch, ok := interp.channels[chanID]; ok {
					ch.hasPending = true
					ch.pendingCount = 1
				}
				interp.channelSenders[chanID] = true
			}
		}
		return Value{}, true
	case "Stop", "Ignore", "Reset":
		return Value{}, true
	case "NotifyContext":
		// Returns (context.Context, context.CancelFunc) — both opaque.
		opaque := stdlibOpaque
		return Value{Raw: []Value{opaque, opaque}}, true
	}
	return Value{}, false
}

// handleZlibCall models compress/zlib.* functions (#91).
func (interp *Interpreter) handleZlibCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader":
		// zlib.NewReader(r io.Reader) (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewReaderDict":
		// zlib.NewReaderDict(r io.Reader, dict []byte) (io.ReadCloser, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewWriter":
		// zlib.NewWriter(w io.Writer) *Writer — single return value.
		return opaque, true

	case "NewWriterLevel":
		// zlib.NewWriterLevel(w io.Writer, level int) (*Writer, error)
		return Value{Raw: []Value{opaque, {}}}, true

	case "NewWriterLevelDict":
		return Value{Raw: []Value{opaque, {}}}, true

	// Shared io.ReadCloser / *Writer methods:
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: "EOF"}}}, true

	case "Close":
		return Value{}, true

	case "Reset":
		return Value{}, true

	case "Write":
		n := 0
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = len(b)
			case []Value:
				n = len(b)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true

	case "Flush":
		return Value{}, true
	}
	_ = opaque
	return Value{}, false
}

// handleBinaryCall models encoding/binary.* functions (#97).
// Read/Write are treated as noops; varint helpers operate on concrete buffers
// when available. ByteOrder method calls (LittleEndian.Uint32 etc.) return
// zero values — they're called on opaque ByteOrder interface values.
func (interp *Interpreter) handleBinaryCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Read":
		// binary.Read(r, order, data interface{}) error — noop, return nil.
		return Value{}, true
	case "Write":
		// binary.Write(w, order, data interface{}) error — noop, return nil.
		return Value{}, true
	case "Size":
		// binary.Size(v interface{}) int — return 8 (a plausible size).
		return Value{Raw: int64(8)}, true

	// Varint encode/decode:
	case "Uvarint":
		// binary.Uvarint(buf []byte) (uint64, int)
		return Value{Raw: []Value{{Raw: uint64(0)}, {Raw: int64(1)}}}, true
	case "Varint":
		// binary.Varint(buf []byte) (int64, int)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(1)}}}, true
	case "PutUvarint":
		// binary.PutUvarint(buf []byte, x uint64) int
		return Value{Raw: int64(1)}, true
	case "PutVarint":
		// binary.PutVarint(buf []byte, x int64) int
		return Value{Raw: int64(1)}, true
	case "AppendUvarint", "AppendVarint":
		// Returns appended []byte.
		if len(args) >= 1 {
			return args[0], true
		}
		return Value{Raw: []Value{}}, true

	// ByteOrder methods (LittleEndian/BigEndian struct method calls):
	case "Uint16":
		return Value{Raw: uint64(0)}, true
	case "Uint32":
		return Value{Raw: uint64(0)}, true
	case "Uint64":
		return Value{Raw: uint64(0)}, true
	case "PutUint16", "PutUint32", "PutUint64":
		return Value{}, true
	case "String":
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleHashExtCall models hash/crc32, hash/fnv, and hash/adler32 (#98).
// All three packages expose the same hash.Hash32/Hash64 interface so method
// calls are handled uniformly regardless of pkgPath.
func (interp *Interpreter) handleHashExtCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors:
	case "New", "NewIEEE", "NewCastagnoli", "NewKoopman",
		"New32", "New32a", "New64", "New64a", "New128", "New128a":
		return opaque, true

	case "MakeTable":
		return opaque, true

	// Package-level checksum helpers:
	case "Checksum", "ChecksumIEEE":
		return Value{Raw: uint64(0)}, true

	// hash.Hash / hash.Hash32 / hash.Hash64 methods (receiver = args[0]):
	case "Write":
		// (n int, err error)
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "Sum":
		// Sum(b []byte) []byte — return the input slice unmodified (pessimistic).
		if len(args) >= 2 {
			return args[1], true
		}
		return Value{Raw: []Value{}}, true
	case "Sum32":
		return Value{Raw: uint64(0)}, true
	case "Sum64":
		return Value{Raw: uint64(0)}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(4)}, true
	case "BlockSize":
		return Value{Raw: int64(64)}, true
	}
	_ = pkgPath
	return Value{}, false
}

// handleContainerCall models container/list, container/heap, and container/ring (#99).
func (interp *Interpreter) handleContainerCall(gid int64, pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch pkgPath {
	case "container/list":
		switch name {
		case "New":
			return opaque, true
		// *List methods returning *Element:
		case "PushFront", "PushBack", "InsertBefore", "InsertAfter":
			return opaque, true
		// *List methods returning nothing meaningful:
		case "Init", "Remove", "MoveToFront", "MoveToBack",
			"MoveBefore", "MoveAfter", "PushFrontList", "PushBackList":
			return Value{}, true
		// *List accessors:
		case "Front", "Back":
			return opaque, true
		case "Len":
			return Value{Raw: int64(0)}, true
		// *Element methods:
		case "Next", "Prev":
			return opaque, true
		}

	case "container/heap":
		switch name {
		case "Init", "Fix":
			return Value{}, true
		case "Push":
			return Value{}, true
		case "Pop", "Remove":
			return opaque, true
		}

	case "container/ring":
		switch name {
		case "New":
			return opaque, true
		case "Next", "Prev":
			return opaque, true
		case "Move":
			return opaque, true
		case "Link", "Unlink":
			return opaque, true
		case "Len":
			return Value{Raw: int64(0)}, true
		case "Do":
			// Do(f func(*Ring)) — probe the callback if possible.
			if len(args) >= 2 {
				sentinel := Value{Raw: struct{}{}}
				switch fn := args[1].Raw.(type) {
				case *ssa.Function:
					if fn.Blocks != nil {
						interp.execFunction(gid, fn, []Value{sentinel})
					}
				case *ClosureValue:
					callArgs := append([]Value{sentinel}, fn.FreeVars...)
					interp.execFunction(gid, fn.Fn, callArgs)
				}
			}
			return Value{}, true
		}
	}
	_ = opaque
	return Value{}, false
}

// handleMathBigCall models math/big.* functions (#100).
// All constructors return opaque values; arithmetic methods return the receiver
// (big.Int methods like Add update and return the receiver); comparison methods
// return pessimistic values.
func (interp *Interpreter) handleMathBigCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors:
	case "NewInt":
		return opaque, true
	case "NewFloat":
		return opaque, true
	case "NewRat":
		return opaque, true

	// Arithmetic / set methods — return receiver (all three types share this pattern):
	case "Add", "Sub", "Mul", "Div", "Mod", "Rem",
		"Quo", "QuoRem",
		"Abs", "Neg", "Inv",
		"And", "Or", "Xor", "AndNot",
		"Lsh", "Rsh", "Not",
		"Exp", "GCD", "Sqrt",
		"Set", "SetInt64", "SetUint64", "SetBytes", "SetBit", "SetBits",
		"SetString", "SetFrac", "SetFrac64",
		"SetFloat64", "SetInt", "SetPrec", "SetMode", "SetMantExp", "SetInf":
		// Return receiver (args[0]) so downstream uses see a non-nil *big.Int/*.Float/*.Rat.
		if len(args) >= 1 {
			return args[0], true
		}
		return opaque, true

	// Value extractors:
	case "Int64":
		return Value{Raw: int64(0)}, true
	case "Uint64":
		return Value{Raw: uint64(0)}, true
	case "IsInt64", "IsUint64":
		return Value{Raw: true}, true
	case "BitLen":
		return Value{Raw: int64(0)}, true
	case "Bit":
		return Value{Raw: uint64(0)}, true
	case "Bytes":
		return Value{Raw: []Value{}}, true
	case "Text", "String":
		return Value{Raw: "0"}, true
	case "Append":
		if len(args) >= 2 {
			return args[1], true
		}
		return Value{Raw: []Value{}}, true
	case "MarshalText", "MarshalJSON":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "UnmarshalText", "UnmarshalJSON":
		return Value{}, true

	// Comparisons:
	case "Cmp", "CmpAbs":
		return Value{Raw: int64(0)}, true
	case "Sign":
		return Value{Raw: int64(1)}, true // pessimistic: positive
	case "ProbablyPrime":
		return Value{Raw: true}, true

	// *big.Float-specific:
	case "Float64":
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: int64(0)}}}, true
	case "Float32":
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: int64(0)}}}, true
	case "Int":
		return Value{Raw: []Value{opaque, {Raw: int64(0)}}}, true
	case "Prec":
		return Value{Raw: uint64(0)}, true
	case "Mode":
		return Value{Raw: int64(0)}, true
	case "Acc":
		return Value{Raw: int64(0)}, true
	case "IsInf", "IsNaN":
		return Value{Raw: false}, true
	case "MinPrec":
		return Value{Raw: uint64(0)}, true

	// *big.Rat-specific:
	case "Num", "Denom":
		return opaque, true
	case "FloatString":
		return Value{Raw: "0"}, true
	case "RatString":
		return Value{Raw: "0/1"}, true
	case "FloatPrec":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true
	case "IsInt":
		return Value{Raw: false}, true
	}
	return Value{}, false
}

// handleTLSCall models crypto/tls.* functions (#101).
// All constructors return opaque values; *Conn methods model the net.Conn interface.
func (interp *Interpreter) handleTLSCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors returning (*Conn, error):
	case "Dial", "DialWithDialer":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Client", "Server":
		// tls.Client/Server(conn net.Conn, config *Config) *tls.Conn
		return opaque, true

	// Listener constructors:
	case "Listen", "NewListener":
		return Value{Raw: []Value{opaque, {}}}, true

	// Certificate loading:
	case "LoadX509KeyPair", "X509KeyPair":
		// (tls.Certificate, error)
		return Value{Raw: []Value{opaque, {}}}, true

	// *tls.Conn methods (receiver = args[0]):
	case "Handshake", "HandshakeContext":
		return Value{}, true // error=nil
	case "ConnectionState":
		return opaque, true
	case "VerifyHostname":
		return Value{}, true
	case "OCSPResponse":
		return Value{Raw: []Value{}}, true
	case "NetConn":
		return opaque, true
	case "RemoteAddr", "LocalAddr":
		return opaque, true
	case "SetDeadline", "SetReadDeadline", "SetWriteDeadline":
		return Value{}, true
	case "Read":
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "Write":
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "Close":
		return Value{}, true

	// *tls.Config helpers:
	case "Clone":
		return opaque, true

	// net.Listener methods on *tls.listener:
	case "Accept":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Addr":
		return opaque, true
	}
	return Value{}, false
}

// handleSQLCall models database/sql.* functions (#102).
// Rows.Next always returns false (no rows in the interpreter's model) so
// the scan loop body is never entered — this is conservative but prevents
// false violations from unterminated loops.
func (interp *Interpreter) handleSQLCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors:
	case "Open":
		// sql.Open(driver, dsn) (*DB, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "OpenDB":
		// sql.OpenDB(connector) *DB
		return opaque, true

	// *sql.DB methods:
	case "Query", "QueryContext":
		// (*Rows, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "QueryRow", "QueryRowContext":
		// *Row
		return opaque, true
	case "Exec", "ExecContext":
		// (Result, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Prepare", "PrepareContext":
		// (*Stmt, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Begin", "BeginTx":
		// (*Tx, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Close", "Ping", "PingContext":
		return Value{}, true
	case "SetMaxOpenConns", "SetMaxIdleConns",
		"SetConnMaxLifetime", "SetConnMaxIdleTime":
		return Value{}, true
	case "Stats":
		return opaque, true
	case "Conn", "ConnContext":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Driver":
		return opaque, true

	// *sql.Rows methods:
	case "Next", "NextResultSet":
		// Return false — no rows in interpreter's model.
		return Value{Raw: false}, true
	case "Scan":
		// nil error — values remain at zero.
		return Value{}, true
	case "Columns":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ColumnTypes":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Err":
		return Value{}, true

	// *sql.Row method:
	// "Scan" is already handled above (same name).

	// *sql.Tx methods:
	case "Commit", "Rollback":
		return Value{}, true

	// *sql.Stmt methods:
	// Exec/Query/QueryRow/Close/ExecContext/QueryContext/QueryRowContext
	// share names with DB methods — handled above.

	// *sql.Result methods:
	case "LastInsertId", "RowsAffected":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true

	// *sql.ColumnType methods:
	case "Name":
		return Value{Raw: ""}, true
	case "DatabaseTypeName":
		return Value{Raw: ""}, true
	case "Length":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true
	case "Nullable":
		return Value{Raw: []Value{{Raw: false}, {Raw: false}}}, true
	case "DecimalSize":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {Raw: false}}}, true
	case "ScanType":
		return opaque, true

	// sql.Named helper:
	case "Named":
		return opaque, true

	// Register helpers (noops):
	case "Register":
		return Value{}, true
	}
	return Value{}, false
}

// handleTestingCall models testing.T/B/F method intercepts (#104).
// Fatal/Fatalf mark the goroutine panicked (like runtime.Goexit).
// Run probes the subtest function with a sentinel *testing.T.
func (interp *Interpreter) handleTestingCall(gid int64, name string, args []Value) (Value, bool) {
	switch name {
	// Logging — noop:
	case "Log", "Logf":
		return Value{}, true
	case "Error", "Errorf":
		return Value{}, true

	// Fatal — stop the current goroutine (like runtime.Goexit):
	case "Fatal", "Fatalf", "FailNow":
		if g, ok := interp.goroutines[gid]; ok {
			g.Panicked = true
		}
		return Value{}, true

	// Skip — noop (don't stop execution in analysis mode):
	case "Skip", "Skipf", "SkipNow":
		return Value{}, true

	// State queries:
	case "Failed", "Skipped":
		return Value{Raw: false}, true
	case "Name":
		return Value{Raw: ""}, true

	// Helpers — noop:
	case "Helper", "Parallel", "Cleanup":
		return Value{}, true

	// TempDir:
	case "TempDir":
		return Value{Raw: "/tmp"}, true

	// Setenv (testing.T.Setenv):
	case "Setenv":
		return Value{}, true

	// Run(name, f) — probe the subtest function:
	case "Run":
		if len(args) >= 3 {
			sentinel := Value{Raw: struct{}{}}
			switch fn := args[2].Raw.(type) {
			case *ssa.Function:
				if fn.Blocks != nil {
					interp.execFunction(gid, fn, []Value{sentinel})
				}
			case *ClosureValue:
				callArgs := append([]Value{sentinel}, fn.FreeVars...)
				interp.execFunction(gid, fn.Fn, callArgs)
			}
		}
		return Value{Raw: true}, true

	// *testing.B specific:
	case "ResetTimer", "StartTimer", "StopTimer", "ReportAllocs",
		"SetBytes", "ReportMetric", "SetParallelism":
		return Value{}, true
	case "N":
		return Value{Raw: int64(1)}, true

	// *testing.F (fuzz):
	case "Add", "Fuzz":
		return Value{}, true

	// Package-level helpers:
	case "Short", "Verbose", "CoverMode":
		return Value{Raw: false}, true
	case "Init":
		return Value{}, true
	}
	return Value{}, false
}

// handleFsCall models io/fs and embed package functions (#109).
// io/fs provides the file-system abstraction; embed.FS implements fs.FS.
func (interp *Interpreter) handleFsCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ReadFile":
		// ReadFile(fsys FS, name string) ([]byte, error) or embed.FS.ReadFile(name).
		return Value{Raw: []Value{{Raw: []byte{0}}, {}}}, true
	case "ReadDir":
		// ReadDir(fsys FS, name string) ([]DirEntry, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Stat":
		// Stat(fsys FS, name string) (FileInfo, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "Open":
		// FS.Open(name string) (File, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "WalkDir":
		// WalkDir(fsys FS, root string, fn WalkDirFunc) error — skip walking.
		return Value{}, true
	case "Glob":
		// Glob(fsys FS, pattern string) ([]string, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Sub":
		// Sub(fsys FS, dir string) (FS, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "ValidPath":
		// ValidPath(name string) bool.
		s, ok := stdlibArgString(args, 0)
		if ok {
			return Value{Raw: len(s) > 0 && s[0] != '/' && s != ".."}, true
		}
		return Value{Raw: true}, true
	case "FileInfoToDirEntry":
		return opaque, true
	// fs.File and fs.DirEntry / fs.FileInfo methods:
	case "Name":
		return Value{Raw: "file"}, true
	case "IsDir":
		return Value{Raw: false}, true
	case "Type":
		// fs.DirEntry.Type() fs.FileMode
		return Value{Raw: int64(0)}, true
	case "Info":
		// fs.DirEntry.Info() (fs.FileInfo, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "ModTime":
		return opaque, true
	case "Mode":
		return Value{Raw: int64(0)}, true
	case "Size":
		return Value{Raw: int64(0)}, true
	case "Sys":
		return Value{Raw: nil}, true
	case "Read":
		// fs.File.Read(b []byte) (int, error)
		n := int64(0)
		if len(args) >= 2 {
			switch b := args[1].Raw.(type) {
			case []byte:
				n = int64(len(b))
			case []Value:
				n = int64(len(b))
			}
		}
		return Value{Raw: []Value{{Raw: n}, {}}}, true
	case "Close":
		return Value{}, true
	}
	_ = pkgPath
	return Value{}, false
}

// handleArchiveCall models archive/zip and archive/tar functions (#110).
func (interp *Interpreter) handleArchiveCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch pkgPath {
	case "archive/zip":
		switch name {
		case "OpenReader":
			// OpenReader(name string) (*ReadCloser, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "NewReader":
			// NewReader(r io.ReaderAt, size int64) (*Reader, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "NewWriter":
			// NewWriter(w io.Writer) *Writer
			return opaque, true
		// *zip.Writer methods:
		case "Create", "CreateHeader", "CreateRaw":
			// (io.Writer, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "Copy", "Close", "Flush":
			return Value{}, true
		case "SetOffset", "SetComment", "RegisterCompressor":
			return Value{}, true
		// *zip.Reader / *zip.ReadCloser methods:
		case "Open":
			return Value{Raw: []Value{opaque, {}}}, true
		case "RegisterDecompressor":
			return Value{}, true
		// *zip.File methods:
		case "FileInfo":
			return opaque, true
		case "Mode":
			return Value{Raw: int64(0)}, true
		case "Modified":
			return opaque, true
		case "DataOffset":
			return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
		}

	case "archive/tar":
		switch name {
		case "NewReader":
			// NewReader(r io.Reader) *Reader
			return opaque, true
		case "NewWriter":
			// NewWriter(w io.Writer) *Writer
			return opaque, true
		// *tar.Reader methods:
		case "Next":
			// (*Header, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "Read":
			n := int64(0)
			if len(args) >= 2 {
				switch b := args[1].Raw.(type) {
				case []byte:
					n = int64(len(b))
				case []Value:
					n = int64(len(b))
				}
			}
			return Value{Raw: []Value{{Raw: n}, {}}}, true
		case "WriteTo":
			return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
		// *tar.Writer methods:
		case "WriteHeader":
			return Value{}, true
		case "Write":
			n := int64(0)
			if len(args) >= 2 {
				switch b := args[1].Raw.(type) {
				case []byte:
					n = int64(len(b))
				case []Value:
					n = int64(len(b))
				}
			}
			return Value{Raw: []Value{{Raw: n}, {}}}, true
		case "Flush", "Close":
			return Value{}, true
		// Helpers:
		case "FileInfoHeader":
			// FileInfoHeader(fi fs.FileInfo, link string) (*Header, error)
			return Value{Raw: []Value{opaque, {}}}, true
		}
	}
	_ = opaque
	return Value{}, false
}

// handleMimeCall models mime and mime/multipart functions (#111).
func (interp *Interpreter) handleMimeCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch pkgPath {
	case "mime":
		switch name {
		case "TypeByExtension":
			// TypeByExtension(ext string) string
			ext, ok := stdlibArgString(args, 0)
			if ok {
				switch ext {
				case ".html", ".htm":
					return Value{Raw: "text/html; charset=utf-8"}, true
				case ".json":
					return Value{Raw: "application/json"}, true
				case ".txt":
					return Value{Raw: "text/plain; charset=utf-8"}, true
				case ".png":
					return Value{Raw: "image/png"}, true
				case ".jpg", ".jpeg":
					return Value{Raw: "image/jpeg"}, true
				}
			}
			return Value{Raw: "application/octet-stream"}, true
		case "ExtensionsByType":
			// ExtensionsByType(typ string) ([]string, error)
			return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
		case "AddExtensionType":
			// AddExtensionType(ext, typ string) error
			return Value{}, true
		case "FormatMediaType":
			// FormatMediaType(t string, param map) string
			s, ok := stdlibArgString(args, 0)
			if ok {
				return Value{Raw: s}, true
			}
			return Value{Raw: "text/plain"}, true
		case "ParseMediaType":
			// ParseMediaType(v string) (mediatype string, params map, err error)
			s, ok := stdlibArgString(args, 0)
			if ok {
				return Value{Raw: []Value{{Raw: s}, {Raw: map[string]interface{}{}}, {}}}, true
			}
			return Value{Raw: []Value{{Raw: "text/plain"}, {Raw: map[string]interface{}{}}, {}}}, true
		case "Encode":
			// WordEncoder.Encode(charset, s string) string
			s, ok := stdlibArgString(args, 1)
			if ok {
				return Value{Raw: s}, true
			}
			return Value{Raw: "=?UTF-8?q?text?="}, true
		case "Decode":
			// WordDecoder.Decode(word string) (string, error)
			s, ok := stdlibArgString(args, 0)
			if ok {
				return Value{Raw: []Value{{Raw: s}, {}}}, true
			}
			return Value{Raw: []Value{{Raw: "text"}, {}}}, true
		case "DecodeHeader":
			// WordDecoder.DecodeHeader(header string) (string, error)
			s, ok := stdlibArgString(args, 0)
			if ok {
				return Value{Raw: []Value{{Raw: s}, {}}}, true
			}
			return Value{Raw: []Value{{Raw: "text"}, {}}}, true
		}

	case "mime/multipart":
		switch name {
		case "NewReader":
			// NewReader(r io.Reader, boundary string) *Reader
			return opaque, true
		case "NewWriter":
			// NewWriter(w io.Writer) *Writer
			return opaque, true
		// *multipart.Reader methods:
		case "NextPart", "NextRawPart":
			// (*Part, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "ReadForm":
			// (*Form, error)
			return Value{Raw: []Value{opaque, {}}}, true
		// *multipart.Writer methods:
		case "CreateFormFile", "CreateFormField", "CreatePart":
			// (io.Writer, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "WriteField":
			// WriteField(fieldname, value string) error
			return Value{}, true
		case "Close":
			return Value{}, true
		case "Boundary":
			return Value{Raw: "boundary1234"}, true
		case "SetBoundary":
			return Value{}, true
		case "FormDataContentType":
			return Value{Raw: "multipart/form-data; boundary=boundary1234"}, true
		// *multipart.Part methods:
		case "Read":
			return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
		case "FileName":
			return Value{Raw: "upload.bin"}, true
		case "FormName":
			return Value{Raw: "field"}, true
		}
	}
	_ = opaque
	return Value{}, false
}

// handleSymCryptoCall models crypto/cipher, crypto/aes, and crypto/hmac (#112).
func (interp *Interpreter) handleSymCryptoCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch pkgPath {
	case "crypto/aes":
		switch name {
		case "NewCipher":
			// NewCipher(key []byte) (cipher.Block, error)
			return Value{Raw: []Value{opaque, {}}}, true
		case "BlockSize":
			return Value{Raw: int64(16)}, true
		}

	case "crypto/hmac":
		switch name {
		case "New":
			// New(h func() hash.Hash, key []byte) hash.Hash
			return opaque, true
		case "Equal":
			// Equal(mac1, mac2 []byte) bool
			if len(args) >= 2 {
				b1, ok1 := args[0].Raw.([]byte)
				b2, ok2 := args[1].Raw.([]byte)
				if ok1 && ok2 && len(b1) == len(b2) {
					eq := true
					for i := range b1 {
						if b1[i] != b2[i] {
							eq = false
							break
						}
					}
					return Value{Raw: eq}, true
				}
			}
			return Value{Raw: false}, true
		// hash.Hash methods on the HMAC result:
		case "Write":
			n := int64(0)
			if len(args) >= 2 {
				switch b := args[1].Raw.(type) {
				case []byte:
					n = int64(len(b))
				case []Value:
					n = int64(len(b))
				}
			}
			return Value{Raw: []Value{{Raw: n}, {}}}, true
		case "Sum":
			if len(args) >= 2 {
				if b, ok := args[1].Raw.([]byte); ok {
					return Value{Raw: b}, true
				}
			}
			return Value{Raw: []byte{}}, true
		case "Reset":
			return Value{}, true
		case "Size":
			return Value{Raw: int64(32)}, true
		case "BlockSize":
			return Value{Raw: int64(64)}, true
		}

	case "crypto/cipher":
		switch name {
		// AEAD construction:
		case "NewGCM", "NewGCMWithNonceSize", "NewGCMWithTagSize":
			// (AEAD, error)
			return Value{Raw: []Value{opaque, {}}}, true
		// Stream cipher construction:
		case "NewCTR", "NewOFB", "NewCFBEncrypter", "NewCFBDecrypter":
			return opaque, true
		// Block cipher modes:
		case "NewCBCEncrypter", "NewCBCDecrypter":
			return opaque, true
		// AEAD.Seal:
		case "Seal":
			// Seal(dst, nonce, plaintext, additionalData []byte) []byte
			if len(args) >= 1 {
				if b, ok := args[0].Raw.([]byte); ok {
					return Value{Raw: append(b, 0)}, true
				}
			}
			return Value{Raw: []byte{0}}, true
		// AEAD.Open:
		case "Open":
			// Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
			if len(args) >= 1 {
				if b, ok := args[0].Raw.([]byte); ok {
					return Value{Raw: []Value{{Raw: b}, {}}}, true
				}
			}
			return Value{Raw: []Value{{Raw: []byte{}}, {}}}, true
		case "NonceSize", "Overhead":
			return Value{Raw: int64(12)}, true
		// BlockMode methods:
		case "BlockSize":
			return Value{Raw: int64(16)}, true
		case "CryptBlocks":
			// CryptBlocks(dst, src []byte) — in-place noop.
			return Value{}, true
		// Stream methods:
		case "XORKeyStream":
			return Value{}, true
		}
	}
	_ = opaque
	return Value{}, false
}

// valuesToInterfaces converts []Value to []interface{} for real fmt.* calls.
// Non-concrete values (Raw == nil) are rendered as "?" to avoid nil-format panics.
func valuesToInterfaces(vals []Value) []interface{} {
	result := make([]interface{}, len(vals))
	for i, v := range vals {
		if v.Raw != nil {
			result[i] = v.Raw
		} else {
			result[i] = "?"
		}
	}
	return result
}

// genericBaseName strips SSA type-parameter suffixes from instantiated generic
// function names, e.g. "Contains[int]" → "Contains", "SortFunc[[]int,int]" → "SortFunc".
// This is needed because Go's SSA (with InstantiateGenerics) appends the concrete
// type arguments to the function name at each call site.
func genericBaseName(name string) string {
	if i := strings.Index(name, "["); i != -1 {
		return name[:i]
	}
	return name
}

// handleSlicesCall models slices.* functions (Go 1.21 generics package, #149).
// For functions accepting a comparison callback, the callback is probed once
// with representative arguments to surface violations inside it.
func (interp *Interpreter) handleSlicesCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback calls the function-value at args[argIdx] with callArgs.
	probeCallback := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	// asSlice returns args[i] as []Value, or nil if not a concrete Giri slice.
	// Giri intercepts (e.g. handleStringsCall) use []Value as a native slice
	// representation; shadow-tracked *SliceValue does not expose element values.
	asSlice := func(i int) []Value {
		if i >= len(args) {
			return nil
		}
		if sv, ok := args[i].Raw.([]Value); ok {
			return sv
		}
		return nil
	}

	switch genericBaseName(name) {
	// ── Predicates ────────────────────────────────────────────────────────────
	case "Contains":
		// slices.Contains(s []E, v E) bool
		if s := asSlice(0); s != nil {
			target := Value{}
			if len(args) >= 2 {
				target = args[1]
			}
			for _, elem := range s {
				if fmt.Sprintf("%v", elem.Raw) == fmt.Sprintf("%v", target.Raw) {
					return Value{Raw: true}, true
				}
			}
			return Value{Raw: false}, true
		}
		return Value{Raw: true}, true // conservative: unknown slice may contain it

	case "ContainsFunc":
		// slices.ContainsFunc(s []E, f func(E) bool) bool
		if s := asSlice(0); s != nil && len(s) > 0 {
			probeCallback(1, []Value{s[0]})
		} else {
			probeCallback(1, []Value{{}})
		}
		return Value{Raw: true}, true

	case "Index":
		// slices.Index(s []E, v E) int
		return Value{Raw: int64(-1)}, true // conservative: not found

	case "IndexFunc":
		// slices.IndexFunc(s []E, f func(E) bool) int
		if s := asSlice(0); s != nil && len(s) > 0 {
			probeCallback(1, []Value{s[0]})
		} else {
			probeCallback(1, []Value{{}})
		}
		return Value{Raw: int64(-1)}, true

	case "Equal":
		// slices.Equal(s1, s2 []E) bool
		return Value{Raw: false}, true // conservative

	case "EqualFunc":
		// slices.EqualFunc(s1, s2 []E, eq func(E1,E2) bool) bool
		probeCallback(2, []Value{{}, {}})
		return Value{Raw: false}, true

	// ── Search ────────────────────────────────────────────────────────────────
	case "BinarySearch":
		// slices.BinarySearch(s []E, target E) (int, bool)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true

	case "BinarySearchFunc":
		// slices.BinarySearchFunc(s []E, target T, cmp func(E,T) int) (int, bool)
		probeCallback(2, []Value{{}, {}})
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true

	// ── Ordering ──────────────────────────────────────────────────────────────
	case "Sort", "SortStable":
		// slices.Sort(s []E) — in-place noop.
		return Value{}, true

	case "SortFunc", "SortStableFunc":
		// slices.SortFunc(s []E, cmp func(a, b E) int)
		probeCallback(1, []Value{{}, {}})
		return Value{}, true

	case "IsSorted":
		return Value{Raw: true}, true

	case "IsSortedFunc":
		probeCallback(1, []Value{{}, {}})
		return Value{Raw: true}, true

	case "Max", "Min":
		// slices.Max/Min(s []E) E — return zero/opaque value.
		return Value{}, true

	case "MaxFunc", "MinFunc":
		probeCallback(1, []Value{{}, {}})
		return Value{}, true

	// ── Transformation ────────────────────────────────────────────────────────
	case "Reverse":
		// In-place noop.
		return Value{}, true

	case "Clone":
		// slices.Clone(s []E) []E — return the input slice.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "Compact":
		// slices.Compact(s []E) []E — noop, return input.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "CompactFunc":
		// slices.CompactFunc(s []E, eq func(E,E) bool) []E
		probeCallback(1, []Value{{}, {}})
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "Clip", "Grow":
		// Capacity adjustments — return input unchanged.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "Insert":
		// slices.Insert(s []E, i int, v ...E) []E
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "Delete", "DeleteFunc":
		// slices.Delete(s []E, i, j int) []E
		if genericBaseName(name) == "DeleteFunc" {
			probeCallback(1, []Value{{}})
		}
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "Replace":
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true

	case "Concat":
		// slices.Concat(slices ...[]E) []E — return first arg or empty.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{Raw: []Value{}}, true

	case "Repeat":
		if len(args) > 0 {
			return args[0], true
		}
		return Value{Raw: []Value{}}, true

	case "Collect":
		// slices.Collect(seq iter.Seq[E]) []E — return empty slice.
		return Value{Raw: []Value{}}, true

	case "AppendSeq":
		if len(args) > 0 {
			return args[0], true
		}
		return Value{Raw: []Value{}}, true

	case "All", "Values", "Backward":
		// slices.All(s []E) iter.Seq2[int,E] etc. — return opaque.
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, true // safe noop for unknown slices functions
}

// handleMapsCall models maps.* functions (Go 1.21 generics package, #150).
func (interp *Interpreter) handleMapsCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback calls the function-value at args[argIdx] with callArgs.
	probeCallback := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	switch genericBaseName(name) {
	case "Keys":
		// maps.Keys returns iter.Seq[K] (a function value) — return opaque non-nil
		// so callers don't nil-deref; actual iteration handled as range-over-func.
		return Value{Raw: struct{}{}}, true

	case "Values":
		// maps.Values returns iter.Seq[V] — opaque iterator.
		return Value{Raw: struct{}{}}, true

	case "Clone":
		// maps.Clone(m map[K]V) map[K]V — return a copy.
		if len(args) > 0 {
			if m, ok := args[0].Raw.(map[interface{}]Value); ok {
				clone := make(map[interface{}]Value, len(m))
				for k, v := range m {
					clone[k] = v
				}
				return Value{Raw: clone, Provenance: args[0].Provenance}, true
			}
			return args[0], true
		}
		return Value{}, true

	case "Copy":
		// maps.Copy(dst, src map[K]V) — merge src into dst in-place.
		if len(args) >= 2 {
			dst, dstOk := args[0].Raw.(map[interface{}]Value)
			src, srcOk := args[1].Raw.(map[interface{}]Value)
			if dstOk && srcOk {
				for k, v := range src {
					dst[k] = v
				}
			}
		}
		return Value{}, true

	case "DeleteFunc":
		// maps.DeleteFunc(m map[K]V, del func(K,V) bool) — probe callback.
		probeCallback(1, []Value{{}, {}})
		return Value{}, true

	case "Equal", "EqualFunc":
		if genericBaseName(name) == "EqualFunc" {
			probeCallback(2, []Value{{}, {}})
		}
		return Value{Raw: false}, true // conservative

	case "Collect":
		// maps.Collect(seq iter.Seq2[K,V]) map[K]V
		return Value{Raw: map[interface{}]Value{}}, true

	case "All":
		// maps.All(m map[K]V) iter.Seq2[K,V] — opaque iterator.
		return Value{Raw: struct{}{}}, true

	case "Insert":
		// maps.Insert(m map[K]V, seq iter.Seq2[K,V])
		return Value{}, true
	}
	return Value{}, true // safe noop
}

// handleCmpCall models cmp.* functions (Go 1.21, #151).
func (interp *Interpreter) handleCmpCall(name string, args []Value) (Value, bool) {
	switch genericBaseName(name) {
	case "Compare":
		// cmp.Compare[T Ordered](x, y T) int
		// Attempt a concrete comparison for numeric types.
		if len(args) >= 2 {
			xi, xok := args[0].Raw.(int64)
			yi, yok := args[1].Raw.(int64)
			if xok && yok {
				switch {
				case xi < yi:
					return Value{Raw: int64(-1)}, true
				case xi > yi:
					return Value{Raw: int64(1)}, true
				default:
					return Value{Raw: int64(0)}, true
				}
			}
			xf, xfok := args[0].Raw.(float64)
			yf, yfok := args[1].Raw.(float64)
			if xfok && yfok {
				switch {
				case xf < yf:
					return Value{Raw: int64(-1)}, true
				case xf > yf:
					return Value{Raw: int64(1)}, true
				default:
					return Value{Raw: int64(0)}, true
				}
			}
			xs, xsok := args[0].Raw.(string)
			ys, ysok := args[1].Raw.(string)
			if xsok && ysok {
				return Value{Raw: int64(strings.Compare(xs, ys))}, true
			}
		}
		return Value{Raw: int64(0)}, true // equal (conservative)

	case "Less":
		// cmp.Less[T Ordered](x, y T) bool
		if len(args) >= 2 {
			xi, xok := args[0].Raw.(int64)
			yi, yok := args[1].Raw.(int64)
			if xok && yok {
				return Value{Raw: xi < yi}, true
			}
			xf, xfok := args[0].Raw.(float64)
			yf, yfok := args[1].Raw.(float64)
			if xfok && yfok {
				return Value{Raw: xf < yf}, true
			}
			xs, xsok := args[0].Raw.(string)
			ys, ysok := args[1].Raw.(string)
			if xsok && ysok {
				return Value{Raw: xs < ys}, true
			}
		}
		return Value{Raw: true}, true // conservative

	case "Or":
		// cmp.Or[T comparable](vals ...T) T — return first non-zero value.
		for _, a := range args {
			if a.Raw != nil && a.Raw != false && a.Raw != int64(0) && a.Raw != "" && a.Raw != 0.0 {
				return a, true
			}
		}
		if len(args) > 0 {
			return args[len(args)-1], true // last arg (zero) if all are zero
		}
		return Value{}, true
	}
	return Value{}, true // safe noop
}

// handleSlogCall models log/slog.* functions (Go 1.21, #152).
// All logging functions are noops for violation analysis purposes.
// Constructors return opaque non-nil values so nil-checks pass.
func (interp *Interpreter) handleSlogCall(name string, args []Value) (Value, bool) {
	switch name {
	// Package-level logging — noops.
	case "Debug", "Info", "Warn", "Error",
		"DebugContext", "InfoContext", "WarnContext", "ErrorContext",
		"Log", "LogAttrs":
		return Value{}, true

	// Default logger access.
	case "Default", "SetDefault":
		return Value{Raw: struct{}{}}, true

	// Logger constructors — return opaque non-nil so method calls on them
	// don't trigger nil-pointer-deref.
	case "New":
		return Value{Raw: struct{}{}}, true

	case "NewTextHandler", "NewJSONHandler":
		// (io.Writer, *HandlerOptions) → Handler
		return Value{Raw: struct{}{}}, true

	// Attribute constructors.
	case "String", "Int", "Int64", "Uint64", "Float64", "Bool",
		"Time", "Duration", "Group", "Any", "AnyValue":
		return Value{Raw: struct{}{}}, true

	case "With":
		// (*Logger).With or slog.With — return opaque Logger.
		return Value{Raw: struct{}{}}, true

	// Logger method calls — all noops or opaque returns.
	case "Handler", "Enabled", "WithGroup":
		return Value{Raw: struct{}{}}, true

	case "Handle", "Enabled2":
		return Value{}, true
	}
	return Value{}, true // safe noop for unknown slog functions
}

// handleIterCall models iter.* functions (Go 1.23, #153).
//
// iter.Pull converts a push-based iterator (iter.Seq[V]) into a pull-based
// iterator returning (next func() (V, bool), stop func()). iter.Pull2 is the
// two-value variant returning (next func() (K, V, bool), stop func()).
//
// In the interpreter, the sequence function argument is opaque (returned by
// slices.All, maps.Keys, etc. as struct{}{}). We return a conservative pair:
// an exhausted next function (immediately returns zero, false) and a noop stop.
// This prevents false positives while providing safe values for any code that
// calls next() or stop().
func (interp *Interpreter) handleIterCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Pull":
		// (seq iter.Seq[V]) → (next func() (V, bool), stop func())
		// Return a pair: opaque non-nil for both next and stop.
		next := Value{Raw: struct{}{}}
		stop := Value{Raw: struct{}{}}
		return Value{Raw: []Value{next, stop}}, true

	case "Pull2":
		// (seq iter.Seq2[K, V]) → (next func() (K, V, bool), stop func())
		next := Value{Raw: struct{}{}}
		stop := Value{Raw: struct{}{}}
		return Value{Raw: []Value{next, stop}}, true
	}
	return Value{}, true // safe noop
}

// handleMathBitsCall models math/bits.* functions (#155).
// All functions operate on concrete uint values; concrete arguments use real
// stdlib calls for accuracy. Non-concrete arguments return a non-zero sentinel.
func (interp *Interpreter) handleMathBitsCall(name string, args []Value) (Value, bool) {
	u0, u0ok := stdlibArgUint(args, 0)
	u1, u1ok := stdlibArgUint(args, 1)

	switch name {
	case "LeadingZeros", "LeadingZeros8", "LeadingZeros16", "LeadingZeros32", "LeadingZeros64":
		if u0ok {
			return Value{Raw: int64(bits.LeadingZeros64(u0))}, true
		}
		return Value{Raw: int64(1)}, true
	case "TrailingZeros", "TrailingZeros8", "TrailingZeros16", "TrailingZeros32", "TrailingZeros64":
		if u0ok {
			return Value{Raw: int64(bits.TrailingZeros64(u0))}, true
		}
		return Value{Raw: int64(1)}, true
	case "OnesCount", "OnesCount8", "OnesCount16", "OnesCount32", "OnesCount64":
		if u0ok {
			return Value{Raw: int64(bits.OnesCount64(u0))}, true
		}
		return Value{Raw: int64(1)}, true
	case "RotateLeft", "RotateLeft8", "RotateLeft16", "RotateLeft32", "RotateLeft64":
		if u0ok && u1ok {
			return Value{Raw: int64(bits.RotateLeft64(u0, int(u1)))}, true
		}
		return Value{Raw: int64(1)}, true
	case "Reverse", "Reverse8", "Reverse16", "Reverse32", "Reverse64":
		if u0ok {
			return Value{Raw: int64(bits.Reverse64(u0))}, true
		}
		return Value{Raw: int64(1)}, true
	case "ReverseBytes", "ReverseBytes16", "ReverseBytes32", "ReverseBytes64":
		if u0ok {
			return Value{Raw: int64(bits.ReverseBytes64(u0))}, true
		}
		return Value{Raw: int64(1)}, true
	case "Len", "Len8", "Len16", "Len32", "Len64":
		if u0ok {
			return Value{Raw: int64(bits.Len64(u0))}, true
		}
		return Value{Raw: int64(1)}, true
	case "UintSize":
		return Value{Raw: int64(bits.UintSize)}, true
	case "Add", "Add32", "Add64":
		// (x, y, carry) → (sum, carryOut)
		if u0ok && u1ok {
			c := uint64(0)
			if len(args) >= 3 {
				if cv, ok := toUint64(args[2]); ok {
					c = cv
				}
			}
			sum, co := bits.Add64(u0, u1, c)
			return Value{Raw: []Value{{Raw: int64(sum)}, {Raw: int64(co)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {Raw: int64(0)}}}, true
	case "Sub", "Sub32", "Sub64":
		if u0ok && u1ok {
			b := uint64(0)
			if len(args) >= 3 {
				if bv, ok := toUint64(args[2]); ok {
					b = bv
				}
			}
			diff, bo := bits.Sub64(u0, u1, b)
			return Value{Raw: []Value{{Raw: int64(diff)}, {Raw: int64(bo)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {Raw: int64(0)}}}, true
	case "Mul", "Mul32", "Mul64":
		if u0ok && u1ok {
			hi, lo := bits.Mul64(u0, u1)
			return Value{Raw: []Value{{Raw: int64(hi)}, {Raw: int64(lo)}}}, true
		}
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(1)}}}, true
	case "Div", "Div32", "Div64":
		// (hi, lo, y) → (quo, rem)
		if len(args) >= 3 {
			if hi, hiok := toUint64(args[0]); hiok {
				if lo, lok := toUint64(args[1]); lok {
					if y, yok := toUint64(args[2]); yok && y != 0 {
						q, r := bits.Div64(hi, lo, y)
						return Value{Raw: []Value{{Raw: int64(q)}, {Raw: int64(r)}}}, true
					}
				}
			}
		}
		return Value{Raw: []Value{{Raw: int64(1)}, {Raw: int64(0)}}}, true
	}
	return Value{}, true
}

// toUint64 converts a Value to uint64, returns (val, ok).
func toUint64(v Value) (uint64, bool) {
	switch x := v.Raw.(type) {
	case int64:
		return uint64(x), true
	case uint64:
		return x, true
	case int:
		return uint64(x), true
	}
	return 0, false
}

// stdlibArgUint extracts the i-th argument as uint64.
func stdlibArgUint(args []Value, i int) (uint64, bool) {
	if i >= len(args) {
		return 0, false
	}
	return toUint64(args[i])
}

// handleMathCmplxCall models math/cmplx.* functions (#155).
// Concrete complex128 arguments use real stdlib calls. Non-concrete args
// return a non-zero opaque complex.
func (interp *Interpreter) handleMathCmplxCall(name string, args []Value) (Value, bool) {
	asComplex := func(i int) (complex128, bool) {
		if i >= len(args) {
			return 0, false
		}
		c, ok := args[i].Raw.(complex128)
		return c, ok
	}
	asFloat := func(i int) (float64, bool) {
		if i >= len(args) {
			return 0, false
		}
		switch x := args[i].Raw.(type) {
		case float64:
			return x, true
		case int64:
			return float64(x), true
		}
		return 0, false
	}

	c0, c0ok := asComplex(0)
	c1, c1ok := asComplex(1)
	f0, f0ok := asFloat(0)
	f1, f1ok := asFloat(1)

	switch name {
	case "Abs":
		if c0ok {
			return Value{Raw: cmplx.Abs(c0)}, true
		}
		return Value{Raw: float64(1)}, true
	case "Phase":
		if c0ok {
			return Value{Raw: cmplx.Phase(c0)}, true
		}
		return Value{Raw: float64(1)}, true
	case "Polar":
		if c0ok {
			r, θ := cmplx.Polar(c0)
			return Value{Raw: []Value{{Raw: r}, {Raw: θ}}}, true
		}
		return Value{Raw: []Value{{Raw: float64(1)}, {Raw: float64(0)}}}, true
	case "Rect":
		if f0ok && f1ok {
			return Value{Raw: cmplx.Rect(f0, f1)}, true
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Conj":
		if c0ok {
			return Value{Raw: cmplx.Conj(c0)}, true
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Exp":
		if c0ok {
			return Value{Raw: cmplx.Exp(c0)}, true
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Log", "Log10":
		if c0ok {
			if name == "Log10" {
				return Value{Raw: cmplx.Log10(c0)}, true
			}
			return Value{Raw: cmplx.Log(c0)}, true
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Sqrt":
		if c0ok {
			return Value{Raw: cmplx.Sqrt(c0)}, true
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Pow":
		if c0ok && c1ok {
			return Value{Raw: cmplx.Pow(c0, c1)}, true
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Sin", "Cos", "Tan":
		if c0ok {
			switch name {
			case "Sin":
				return Value{Raw: cmplx.Sin(c0)}, true
			case "Cos":
				return Value{Raw: cmplx.Cos(c0)}, true
			case "Tan":
				return Value{Raw: cmplx.Tan(c0)}, true
			}
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Sinh", "Cosh", "Tanh":
		if c0ok {
			switch name {
			case "Sinh":
				return Value{Raw: cmplx.Sinh(c0)}, true
			case "Cosh":
				return Value{Raw: cmplx.Cosh(c0)}, true
			case "Tanh":
				return Value{Raw: cmplx.Tanh(c0)}, true
			}
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "Asin", "Acos", "Atan", "Asinh", "Acosh", "Atanh":
		if c0ok {
			switch name {
			case "Asin":
				return Value{Raw: cmplx.Asin(c0)}, true
			case "Acos":
				return Value{Raw: cmplx.Acos(c0)}, true
			case "Atan":
				return Value{Raw: cmplx.Atan(c0)}, true
			case "Asinh":
				return Value{Raw: cmplx.Asinh(c0)}, true
			case "Acosh":
				return Value{Raw: cmplx.Acosh(c0)}, true
			case "Atanh":
				return Value{Raw: cmplx.Atanh(c0)}, true
			}
		}
		return Value{Raw: complex(float64(1), float64(0))}, true
	case "IsNaN":
		if c0ok {
			return Value{Raw: cmplx.IsNaN(c0)}, true
		}
		return Value{Raw: false}, true
	case "IsInf":
		if c0ok {
			return Value{Raw: cmplx.IsInf(c0)}, true
		}
		return Value{Raw: false}, true
	case "NaN":
		return Value{Raw: cmplx.NaN()}, true
	case "Inf":
		return Value{Raw: cmplx.Inf()}, true
	}
	return Value{}, true
}

// handleHTMLCall models html.* functions (#156).
// EscapeString and UnescapeString use real stdlib for concrete string args.
func (interp *Interpreter) handleHTMLCall(name string, args []Value) (Value, bool) {
	s0, s0ok := stdlibArgString(args, 0)
	switch name {
	case "EscapeString":
		if s0ok {
			return Value{Raw: html.EscapeString(s0)}, true
		}
		return Value{Raw: "&amp;sentinel"}, true
	case "UnescapeString":
		if s0ok {
			return Value{Raw: html.UnescapeString(s0)}, true
		}
		return Value{Raw: "sentinel"}, true
	}
	return Value{}, true
}

// handleUTF16Call models unicode/utf16.* functions (#156).
// Encode/Decode convert between rune slices and UTF-16 uint16 slices.
func (interp *Interpreter) handleUTF16Call(name string, args []Value) (Value, bool) {
	switch name {
	case "IsSurrogate":
		// IsSurrogate(r rune) bool
		if len(args) >= 1 {
			if r, ok := args[0].Raw.(int64); ok {
				return Value{Raw: utf16.IsSurrogate(rune(r))}, true
			}
		}
		return Value{Raw: false}, true

	case "EncodeRune":
		// EncodeRune(r rune) (r1, r2 rune)
		if len(args) >= 1 {
			if r, ok := args[0].Raw.(int64); ok {
				r1, r2 := utf16.EncodeRune(rune(r))
				return Value{Raw: []Value{{Raw: int64(r1)}, {Raw: int64(r2)}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: int64(0xD800)}, {Raw: int64(0xDC00)}}}, true

	case "DecodeRune":
		// DecodeRune(r1, r2 rune) rune
		if len(args) >= 2 {
			r1, r1ok := args[0].Raw.(int64)
			r2, r2ok := args[1].Raw.(int64)
			if r1ok && r2ok {
				return Value{Raw: int64(utf16.DecodeRune(rune(r1), rune(r2)))}, true
			}
		}
		return Value{Raw: int64(0x10000)}, true

	case "Encode":
		// Encode(s []rune) []uint16 — return opaque non-nil slice.
		return Value{Raw: []Value{}}, true

	case "Decode":
		// Decode(s []uint16) []rune — return opaque non-nil slice.
		return Value{Raw: []Value{}}, true

	case "AppendRune":
		// AppendRune(a []uint16, r rune) []uint16
		return Value{Raw: []Value{}}, true
	}
	return Value{}, true
}

// handleOSUserCall models os/user.* functions (#157).
// All lookup functions return (opaque-User, nil) or (opaque-Group, nil) since
// the actual user database is not available during interpretation.
func (interp *Interpreter) handleOSUserCall(name string, args []Value) (Value, bool) {
	opaqueUser := Value{Raw: struct{}{}}
	opaqueGroup := Value{Raw: struct{}{}}
	switch name {
	case "Current":
		// () → (*User, error)
		return Value{Raw: []Value{opaqueUser, {}}}, true
	case "Lookup", "LookupId":
		// (string) → (*User, error)
		return Value{Raw: []Value{opaqueUser, {}}}, true
	case "LookupGroup", "LookupGroupId":
		// (string) → (*Group, error)
		return Value{Raw: []Value{opaqueGroup, {}}}, true
	}
	// Method calls on *User or *Group — return opaque non-nil strings.
	return Value{Raw: struct{}{}}, true
}

// handleRuntimeDebugCall models runtime/debug.* functions (#157).
// Heap-profiling and GC functions are noops; Stack returns a non-empty byte slice.
func (interp *Interpreter) handleRuntimeDebugCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Stack":
		// Stack() []byte — return non-empty sentinel.
		return Value{Raw: []Value{{Raw: int64('g')}}}, true

	case "PrintStack":
		// PrintStack() — noop.
		return Value{}, true

	case "SetGCPercent":
		// SetGCPercent(percent int) int — return previous value (100 = default).
		return Value{Raw: int64(100)}, true

	case "SetMemoryLimit":
		// SetMemoryLimit(limit int64) int64
		return Value{Raw: int64(math.MaxInt64)}, true

	case "FreeOSMemory":
		return Value{}, true

	case "ReadGCStats":
		// ReadGCStats(*GCStats) — noop (stats struct remains zero).
		return Value{}, true

	case "ReadMemStats":
		// runtime.ReadMemStats — noop.
		return Value{}, true

	case "SetMaxStack":
		// SetMaxStack(bytes int) int — return previous (1 GiB default).
		return Value{Raw: int64(1 << 30)}, true

	case "SetMaxThreads":
		return Value{Raw: int64(10000)}, true

	case "SetPanicOnFault":
		// SetPanicOnFault(enabled bool) bool
		return Value{Raw: false}, true

	case "WriteHeapDump":
		return Value{}, true

	case "SetTraceback":
		return Value{}, true

	case "ParseBuildInfo":
		// () → (*BuildInfo, error)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "ReadBuildInfo":
		// () → (*BuildInfo, bool)
		return Value{Raw: []Value{{Raw: struct{}{}}, {Raw: true}}}, true

	case "GCStats", "ClearMutexProfile", "SetCPUProfileRate",
		"StartCPUProfile", "StopCPUProfile":
		return Value{}, true
	}
	return Value{}, true // safe noop
}

// handleNetNetipCall models net/netip.* functions (Go 1.18+, #158).
// Constructors return opaque non-nil address values; predicate/accessor methods
// return conservative defaults (IsValid=true, IsUnspecified/IsLoopback=false).
func (interp *Interpreter) handleNetNetipCall(name string, args []Value) (Value, bool) {
	opaqueAddr := Value{Raw: struct{}{}}
	opaqueAddrPort := Value{Raw: struct{}{}}
	opaquePrefix := Value{Raw: struct{}{}}

	switch name {
	// ---- Addr constructors ----
	case "ParseAddr", "MustParseAddr":
		if name == "ParseAddr" {
			return Value{Raw: []Value{opaqueAddr, {}}}, true
		}
		return opaqueAddr, true
	case "AddrFrom4":
		return opaqueAddr, true
	case "AddrFrom16":
		return opaqueAddr, true
	case "AddrFromSlice":
		return Value{Raw: []Value{opaqueAddr, {Raw: true}}}, true
	case "IPv4Unspecified", "IPv6Unspecified", "IPv6LinkLocalAllNodes",
		"IPv6LinkLocalAllRouters", "IPv6Loopback":
		return opaqueAddr, true

	// ---- Addr methods ----
	case "IsValid":
		return Value{Raw: true}, true
	case "IsUnspecified", "IsLoopback", "IsMulticast", "IsLinkLocalUnicast",
		"IsLinkLocalMulticast", "IsInterfaceLocalMulticast", "IsPrivate",
		"Is4", "Is4In6", "Is6", "IsGlobalUnicast":
		return Value{Raw: false}, true
	case "Unmap":
		return opaqueAddr, true
	case "As4":
		return Value{Raw: []Value{{}, {}, {}, {}}}, true
	case "As16":
		return Value{Raw: []Value{{}, {}, {}, {}, {}, {}, {}, {},
			{}, {}, {}, {}, {}, {}, {}, {}}}, true
	case "AsSlice":
		return Value{Raw: []Value{}}, true
	case "BitLen":
		return Value{Raw: int64(32)}, true
	case "Zone":
		return Value{Raw: ""}, true
	case "WithZone":
		return opaqueAddr, true
	case "Compare":
		return Value{Raw: int64(0)}, true
	case "Less":
		return Value{Raw: false}, true
	case "MarshalText", "MarshalBinary":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "UnmarshalText", "UnmarshalBinary":
		return Value{Raw: []Value{{}}}, true
	case "String":
		return Value{Raw: "0.0.0.0"}, true
	case "AppendTo":
		return Value{Raw: []Value{}}, true

	// ---- AddrPort constructors ----
	case "AddrPortFrom":
		return opaqueAddrPort, true
	case "ParseAddrPort", "MustParseAddrPort":
		if name == "ParseAddrPort" {
			return Value{Raw: []Value{opaqueAddrPort, {}}}, true
		}
		return opaqueAddrPort, true
	case "Addr":
		return opaqueAddr, true
	case "Port":
		return Value{Raw: int64(0)}, true

	// ---- Prefix constructors ----
	case "PrefixFrom":
		return opaquePrefix, true
	case "ParsePrefix", "MustParsePrefix":
		if name == "ParsePrefix" {
			return Value{Raw: []Value{opaquePrefix, {}}}, true
		}
		return opaquePrefix, true
	case "Bits":
		return Value{Raw: int64(32)}, true
	case "Masked":
		return opaquePrefix, true
	case "Contains":
		return Value{Raw: false}, true
	case "Overlaps":
		return Value{Raw: false}, true
	case "IsSingleIP":
		return Value{Raw: false}, true
	}
	return Value{}, true
}

// handleMathRandV2Call models math/rand/v2.* functions (Go 1.22+, #153).
// The v2 API eliminates the global source and uses generic N[T] helpers.
// Concrete integer/float arguments use real stdlib calls via math/rand for
// compatibility; non-concrete args return non-zero sentinels.
func (interp *Interpreter) handleMathRandV2Call(name string, args []Value) (Value, bool) {
	switch name {
	// Constructors — return opaque non-nil *Rand equivalent.
	case "New", "NewChaCha8", "NewPCG":
		return Value{Raw: struct{}{}}, true

	// Scalar generators — delegate to interp.rng (seeded from config).
	case "Int", "Int32", "Int64", "Uint", "Uint32", "Uint64":
		return Value{Raw: interp.rng.Int63()}, true
	case "Float32":
		return Value{Raw: interp.rng.Float64()}, true
	case "Float64":
		return Value{Raw: interp.rng.Float64()}, true

	// Bounded generators: Intn / IntN / Int32N / Int64N / UintN / Uint32N / Uint64N / N.
	case "Intn", "IntN", "Int32N", "Int64N", "UintN", "Uint32N", "Uint64N", "N":
		if len(args) >= 1 {
			if n, ok := args[0].Raw.(int64); ok && n > 0 {
				return Value{Raw: interp.rng.Int63n(n)}, true
			}
		}
		return Value{Raw: int64(1)}, true

	// Shuffle: no-op (we don't model slice contents).
	case "Shuffle":
		return Value{}, true

	// Perm: return opaque non-nil slice.
	case "Perm":
		return Value{Raw: []Value{}}, true

	// Source/global reads.
	case "Read":
		return Value{Raw: []Value{{Raw: int64(1)}, {}}}, true

	// *Rand method calls — same returns as package-level functions.
	case "Rand.Int", "Rand.Int64", "Rand.Uint64", "Rand.Float64", "Rand.Float32":
		return Value{Raw: interp.rng.Float64()}, true
	}
	return Value{}, true
}

// handleEncodingPEMCall models encoding/pem.* functions (#154).
// Decode returns the first PEM block parsed from concrete input; Encode/
// EncodeToMemory return non-empty byte slices.
func (interp *Interpreter) handleEncodingPEMCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Decode":
		// Decode(data []byte) (p *Block, rest []byte)
		// Try real decode on concrete bytes; otherwise return opaque block + nil rest.
		if len(args) >= 1 {
			var data []byte
			switch v := args[0].Raw.(type) {
			case []byte:
				data = v
			case string:
				data = []byte(v)
			case []Value:
				for _, b := range v {
					if bv, ok := b.Raw.(int64); ok {
						data = append(data, byte(bv))
					}
				}
			}
			if len(data) > 0 {
				block, rest := pem.Decode(data)
				if block != nil {
					return Value{Raw: []Value{
						{Raw: struct{}{}}, // opaque *Block
						{Raw: rest},
					}}, true
				}
			}
		}
		// No concrete data or not valid PEM — return (opaque, nil).
		return Value{Raw: []Value{{Raw: struct{}{}}, {}}}, true

	case "Encode":
		// Encode(out io.Writer, b *Block) error — noop.
		return Value{}, true

	case "EncodeToMemory":
		// EncodeToMemory(b *Block) []byte — return non-empty sentinel.
		return Value{Raw: []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----\n")}, true
	}
	return Value{}, true
}

// handleEncodingASN1Call models encoding/asn1.* functions (#154).
func (interp *Interpreter) handleEncodingASN1Call(name string, args []Value) (Value, bool) {
	switch name {
	case "Unmarshal":
		// Unmarshal(b []byte, v interface{}) (rest []byte, err error)
		if len(args) >= 2 {
			var data []byte
			switch v := args[0].Raw.(type) {
			case []byte:
				data = v
			case []Value:
				for _, b := range v {
					if bv, ok := b.Raw.(int64); ok {
						data = append(data, byte(bv))
					}
				}
			}
			if len(data) > 0 {
				// Attempt real unmarshal into a RawValue to get rest bytes.
				var raw asn1.RawValue
				rest, err := asn1.Unmarshal(data, &raw)
				var errVal Value
				if err != nil {
					errVal = Value{Raw: err}
				}
				return Value{Raw: []Value{{Raw: rest}, errVal}}, true
			}
		}
		return Value{Raw: []Value{{Raw: []byte{}}, {}}}, true

	case "UnmarshalWithParams":
		return Value{Raw: []Value{{Raw: []byte{}}, {}}}, true

	case "Marshal":
		// Marshal(val interface{}) ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte{0x30, 0x00}}, {}}}, true

	case "MarshalWithParams":
		return Value{Raw: []Value{{Raw: []byte{0x30, 0x00}}, {}}}, true

	case "ObjectIdentifier":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, true
}

// handleCryptoRSACall models crypto/rsa.* functions (#155).
// Key generation and cryptographic operations return opaque values with nil
// errors so programs that check err == nil proceed normally.
func (interp *Interpreter) handleCryptoRSACall(name string, args []Value) (Value, bool) {
	opaqueKey := Value{Raw: struct{}{}}
	switch name {
	case "GenerateKey", "GenerateMultiPrimeKey":
		// (rand io.Reader, bits int) → (*PrivateKey, error)
		return Value{Raw: []Value{opaqueKey, {}}}, true

	case "SignPSS", "SignPKCS1v15":
		// (...) → ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte{0x30, 0x44}}, {}}}, true

	case "VerifyPSS", "VerifyPKCS1v15":
		// (...) → error — return nil (conservative: assume valid)
		return Value{}, true

	case "EncryptOAEP", "EncryptPKCS1v15":
		return Value{Raw: []Value{{Raw: []byte{0x00}}, {}}}, true

	case "DecryptOAEP", "DecryptPKCS1v15", "DecryptPKCS1v15SessionKey":
		return Value{Raw: []Value{{Raw: []byte{0x00}}, {}}}, true

	case "SignPSS_SaltLength", "PSSSaltLength":
		return Value{Raw: int64(32)}, true
	}
	return Value{Raw: struct{}{}}, true
}

// handleCryptoECDSACall models crypto/ecdsa.* functions (#155).
func (interp *Interpreter) handleCryptoECDSACall(name string, args []Value) (Value, bool) {
	opaqueKey := Value{Raw: struct{}{}}
	switch name {
	case "GenerateKey":
		// (c elliptic.Curve, rand io.Reader) → (*PrivateKey, error)
		return Value{Raw: []Value{opaqueKey, {}}}, true

	case "Sign":
		// (rand io.Reader, priv *PrivateKey, hash []byte) → (r, s *big.Int, error)
		bigInt := Value{Raw: struct{}{}}
		return Value{Raw: []Value{bigInt, bigInt, {}}}, true

	case "SignASN1":
		// (rand io.Reader, priv *PrivateKey, hash []byte) → ([]byte, error)
		return Value{Raw: []Value{{Raw: []byte{0x30, 0x44}}, {}}}, true

	case "Verify":
		// (pub *PublicKey, hash []byte, r, s *big.Int) → bool
		// Conservative: return false to avoid false "verification passed" paths.
		return Value{Raw: false}, true

	case "VerifyASN1":
		return Value{Raw: false}, true
	}
	return Value{Raw: struct{}{}}, true
}

// handleCryptoEd25519Call models crypto/ed25519.* functions (#155).
func (interp *Interpreter) handleCryptoEd25519Call(name string, args []Value) (Value, bool) {
	opaqueKey := Value{Raw: struct{}{}}
	switch name {
	case "GenerateKey":
		// (rand io.Reader) → (PublicKey, PrivateKey, error)
		return Value{Raw: []Value{opaqueKey, opaqueKey, {}}}, true

	case "NewKeyFromSeed":
		// (seed []byte) → PrivateKey
		return opaqueKey, true

	case "Sign":
		// (priv PrivateKey, message []byte) → []byte
		return Value{Raw: []byte{0x00, 0x01}}, true

	case "Verify", "VerifyWithOptions":
		// (pub PublicKey, message, sig []byte) → bool / error
		if name == "Verify" {
			return Value{Raw: false}, true
		}
		return Value{}, true // nil error (no panic)
	}
	return Value{Raw: struct{}{}}, true
}

// handleCryptoECDHCall models crypto/ecdh.* functions (Go 1.20+, #155).
func (interp *Interpreter) handleCryptoECDHCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Curve constructors.
	case "P256", "P384", "P521", "X25519":
		return opaque, true

	// Key generation / parsing.
	case "GenerateKey":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewPrivateKey", "NewPublicKey":
		return Value{Raw: []Value{opaque, {}}}, true

	// Key methods.
	case "ECDH":
		return Value{Raw: []Value{opaque, {}}}, true
	case "PublicKey", "Bytes", "Equal":
		return opaque, true
	}
	return opaque, true
}

// handleCryptoX509Call models crypto/x509.* functions (#156 proxy — #155 group).
// Certificate parsing returns an opaque non-nil *Certificate with nil error.
func (interp *Interpreter) handleCryptoX509Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	opaqueSlice := Value{Raw: []Value{}}
	switch name {
	// Certificate parsing.
	case "ParseCertificate", "ParseCertificateRequest":
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParseCertificates":
		return Value{Raw: []Value{opaqueSlice, {}}}, true

	// Key parsing.
	case "ParsePKCS1PrivateKey", "ParsePKCS8PrivateKey",
		"ParseECPrivateKey", "ParsePKCS1PublicKey", "ParsePKIXPublicKey":
		return Value{Raw: []Value{opaque, {}}}, true

	// Key marshaling.
	case "MarshalPKCS1PrivateKey", "MarshalPKCS8PrivateKey",
		"MarshalECPrivateKey", "MarshalPKCS1PublicKey", "MarshalPKIXPublicKey":
		return Value{Raw: []Value{{Raw: []byte{0x30, 0x00}}, {}}}, true

	// Certificate pool.
	case "NewCertPool", "SystemCertPool":
		if name == "SystemCertPool" {
			return Value{Raw: []Value{opaque, {}}}, true
		}
		return opaque, true
	case "AddCert", "AppendCertsFromPEM":
		return Value{Raw: false}, true

	// Verify.
	case "Verify", "VerifyHostname":
		return Value{}, true // nil error

	// Create certificate.
	case "CreateCertificate", "CreateCertificateRequest":
		return Value{Raw: []Value{{Raw: []byte{0x30, 0x00}}, {}}}, true

	// OID / misc.
	case "OIDFromInts", "ParseOID":
		return Value{Raw: []Value{opaque, {}}}, true

	// Certificate methods.
	case "IsCA", "CheckSignatureFrom", "Leaf":
		return Value{Raw: false}, true
	}
	return opaque, true
}

// handleRuntimePprofCall models runtime/pprof.* functions (#156).
// All profiling operations are noops; Lookup returns an opaque non-nil Profile.
func (interp *Interpreter) handleRuntimePprofCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Lookup":
		// Lookup(name string) *Profile — return opaque non-nil.
		return Value{Raw: struct{}{}}, true
	case "Profiles":
		return Value{Raw: []Value{}}, true
	case "StartCPUProfile":
		return Value{}, true // nil error
	case "StopCPUProfile", "WriteHeapProfile":
		return Value{}, true
	case "Do":
		// Do(ctx, Labels, f) — invoke f with ctx.
		return Value{}, true
	case "NewProfile":
		return Value{Raw: struct{}{}}, true
	case "Profile.WriteTo", "Profile.Name", "Profile.Count":
		return Value{}, true
	}
	return Value{}, true
}

// handleRuntimeTraceCall models runtime/trace.* functions (#156).
// All tracing operations are noops; Task/Region constructors return opaque values.
func (interp *Interpreter) handleRuntimeTraceCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Start":
		return Value{}, true // nil error
	case "Stop":
		return Value{}, true
	case "NewTask":
		// NewTask(ctx, taskType) (context.Context, *Task)
		return Value{Raw: []Value{{Raw: struct{}{}}, {Raw: struct{}{}}}}, true
	case "NewRegion":
		// NewRegion(ctx, regionType) *Region
		return Value{Raw: struct{}{}}, true
	case "Log", "Logf":
		return Value{}, true
	case "IsEnabled":
		return Value{Raw: false}, true
	case "WithRegion":
		return Value{}, true
	}
	return Value{}, true
}

// handleErrgroupCall models golang.org/x/sync/errgroup.* functions (#157).
// Group.Go probes the callback once synchronously (like sort.Slice) so that
// violations inside goroutine bodies are still detected. Group.Wait returns nil.
func (interp *Interpreter) handleErrgroupCall(gid int64, name string, args []Value, site string) (Value, bool) {
	// probeCallback invokes the function-value at args[argIdx] once with callArgs.
	probe := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	switch name {
	case "WithContext":
		// WithContext(ctx) → (*Group, context.Context)
		return Value{Raw: []Value{{Raw: struct{}{}}, {Raw: struct{}{}}}}, true

	case "Go", "TryGo":
		// Go(fn func()) — probe fn once with no args (fn takes no params).
		probe(len(args)-1, nil)
		if name == "TryGo" {
			return Value{Raw: true}, true
		}
		return Value{}, true

	case "Wait":
		return Value{}, true // nil error

	case "SetLimit", "GOMAXPROCS":
		return Value{}, true
	}
	return Value{Raw: struct{}{}}, true
}

// handleSingleflightCall models golang.org/x/sync/singleflight.* (#157).
// Group.Do probes the function once and returns (opaque, nil, false).
func (interp *Interpreter) handleSingleflightCall(gid int64, name string, args []Value, site string) (Value, bool) {
	probe := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}

	switch name {
	case "Do":
		// Do(key string, fn func() (interface{}, error)) — fn takes no params.
		probe(len(args)-1, nil)
		return Value{Raw: []Value{{Raw: struct{}{}}, {}, {Raw: false}}}, true

	case "DoChan":
		probe(len(args)-1, nil)
		return Value{Raw: struct{}{}}, true

	case "Forget":
		return Value{}, true
	}
	return Value{Raw: struct{}{}}, true
}

// handleEncodingGobCall models encoding/gob.* functions (#158).
// The gob format is Go-specific binary serialization; Encode/Decode are noops
// since we cannot model the binary wire format during interpretation.
func (interp *Interpreter) handleEncodingGobCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewEncoder":
		return opaque, true
	case "NewDecoder":
		return opaque, true
	case "Register", "RegisterName":
		return Value{}, true
	// Encoder/Decoder methods:
	case "Encode", "EncodeValue":
		return Value{}, true // nil error
	case "Decode", "DecodeValue":
		return Value{}, true // nil error
	}
	return Value{}, true
}

// handleEncodingBase32Call models encoding/base32.* functions (#158).
// StdEncoding and HexEncoding are opaque; concrete string args use real stdlib.
func (interp *Interpreter) handleEncodingBase32Call(name string, args []Value) (Value, bool) {
	// Encoding constants — return opaque Encoding object.
	switch name {
	case "StdEncoding", "HexEncoding":
		return Value{Raw: struct{}{}}, true
	case "NewEncoding":
		return Value{Raw: struct{}{}}, true

	// Encoding methods:
	case "EncodeToString":
		if len(args) >= 1 {
			var data []byte
			switch v := args[len(args)-1].Raw.(type) {
			case []byte:
				data = v
			case string:
				data = []byte(v)
			}
			if len(data) > 0 {
				return Value{Raw: base32.StdEncoding.EncodeToString(data)}, true
			}
		}
		return Value{Raw: "AAAA"}, true // non-empty sentinel

	case "DecodeString":
		if len(args) >= 1 {
			if s, ok := args[len(args)-1].Raw.(string); ok && s != "" {
				b, err := base32.StdEncoding.DecodeString(s)
				if err == nil {
					return Value{Raw: []Value{{Raw: b}, {}}}, true
				}
			}
		}
		return Value{Raw: []Value{{Raw: []byte{0x00}}, {}}}, true

	case "EncodedLen", "DecodedLen":
		return Value{Raw: int64(8)}, true

	case "Encode", "Decode", "AppendEncode", "AppendDecode":
		return Value{}, true

	case "WithPadding":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, true
}

// handleImageCall models image.*, image/color.*, and image/draw.* (#159).
// Image constructors return opaque non-nil *Image values; geometry functions
// return concrete results.
func (interp *Interpreter) handleImageCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch pkgPath {
	case "image":
		switch name {
		case "NewRGBA", "NewNRGBA", "NewRGBA64", "NewNRGBA64",
			"NewAlpha", "NewAlpha16", "NewGray", "NewGray16",
			"NewCMYK", "NewPaletted", "NewUniform":
			return opaque, true
		case "Decode", "DecodeConfig":
			return Value{Raw: []Value{opaque, {}}}, true
		case "Rect":
			// Rect(x0, y0, x1, y1 int) Rectangle
			return opaque, true
		case "Pt":
			// Pt(x, y int) Point
			return opaque, true
		case "RegisterFormat":
			return Value{}, true
		// Image method calls:
		case "Bounds", "ColorModel", "At", "Set", "SubImage", "Opaque",
			"PixOffset", "RGBAAt", "NRGBAAt", "GrayAt":
			return opaque, true
		}
	case "image/color":
		switch name {
		case "RGBAModel", "RGBA64Model", "NRGBAModel", "NRGBA64Model",
			"AlphaModel", "Alpha16Model", "GrayModel", "Gray16Model",
			"CMYKModel", "NYCbCrAModel":
			return opaque, true
		case "ModelFunc":
			return opaque, true
		case "RGBToYCbCr", "YCbCrToRGB", "RGBToCMYK", "CMYKToRGB":
			return opaque, true
		// Color method calls:
		case "RGBA":
			return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {Raw: int64(0)}, {Raw: int64(0xFFFF)}}}, true
		case "Convert":
			return opaque, true
		}
	case "image/draw":
		switch name {
		case "Draw", "DrawMask":
			return Value{}, true
		case "FloydSteinberg":
			return opaque, true
		}
	}
	return opaque, true
}

// handleImageCodecCall models image/png, image/jpeg, image/gif (#159).
func (interp *Interpreter) handleImageCodecCall(pkgPath, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Decode", "DecodeConfig":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Encode":
		return Value{}, true // nil error
	case "EncodeAll": // gif
		return Value{}, true
	case "DecodeAll": // gif
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return opaque, true
}

// handleExpvarCall models expvar.* functions (#160).
// All published variables are opaque; Int/Float/String/Map add accessors.
func (interp *Interpreter) handleExpvarCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewInt", "NewFloat", "NewString", "NewMap":
		return opaque, true
	case "Get":
		return opaque, true
	case "Publish":
		return Value{}, true
	case "Do":
		return Value{}, true
	// *Int / *Float / *String / *Map methods:
	case "Add", "Set", "Value", "String":
		return Value{Raw: int64(0)}, true
	case "Init", "AddFloat":
		return opaque, true
	case "Delete", "Each":
		return Value{}, true
	}
	return Value{}, true
}

// handleTabwriterCall models text/tabwriter.* functions (#160).
// Writer is opaque; Write/Flush are noops.
func (interp *Interpreter) handleTabwriterCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewWriter":
		return opaque, true
	case "Init":
		return opaque, true
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Flush":
		return Value{}, true // nil error
	}
	return Value{}, true
}

// handleTextScannerCall models text/scanner.* functions (#160).
// Scanner is stateful but opaque; Scan returns a non-zero token type sentinel.
func (interp *Interpreter) handleTextScannerCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Init":
		return Value{Raw: struct{}{}}, true
	case "Scan":
		// Returns rune (token type); use EOF sentinel (-1) so loops terminate.
		return Value{Raw: int64(-1)}, true
	case "Peek":
		return Value{Raw: int64(-1)}, true
	case "TokenText":
		return Value{Raw: ""}, true
	case "Pos", "Position":
		return Value{Raw: struct{}{}}, true
	case "IsIdentRune":
		return Value{Raw: false}, true
	}
	return Value{}, true
}

// handleNetSMTPCall models net/smtp.* functions (#162).
func (interp *Interpreter) handleNetSMTPCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Dial", "NewClient":
		// Returns (*Client, error) — opaque client, nil error.
		return Value{Raw: []Value{opaque, {}}}, true
	case "SendMail":
		// Returns error — nil (sent successfully).
		return Value{}, true
	case "PlainAuth", "CRAMMD5Auth":
		// Returns Auth interface — opaque.
		return opaque, true
	// *Client methods
	case "Auth", "Mail", "Rcpt", "Data", "Quit", "Close", "Reset",
		"Noop", "Verify", "Hello", "StartTLS":
		return Value{}, true // nil error
	case "Extension":
		return Value{Raw: []Value{{Raw: false}, {Raw: ""}}}, true
	}
	return Value{}, true
}

// handleNetMailCall models net/mail.* functions (#162).
func (interp *Interpreter) handleNetMailCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ParseAddress":
		// Returns (*Address, error) — opaque address, nil error.
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParseAddressList":
		// Returns ([]*Address, error) — empty list, nil error.
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "NewReader":
		return opaque, true
	case "ReadMessage":
		// Returns (*Message, error) — opaque message, nil error.
		return Value{Raw: []Value{opaque, {}}}, true
	// *Address methods
	case "String":
		return Value{Raw: "<mail.Address>"}, true
	}
	return Value{}, true
}

// handleNetTextprotoCall models net/textproto.* functions (#162).
func (interp *Interpreter) handleNetTextprotoCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewConn":
		return opaque, true
	case "NewReader":
		return opaque, true
	case "NewWriter":
		return opaque, true
	case "CanonicalMIMEHeaderKey":
		// Return concrete passthrough for string args.
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: strings.ToTitle(s[:1]) + strings.ToLower(s[1:])}, true
			}
		}
		return Value{Raw: "<textproto.Key>"}, true
	case "TrimString":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: strings.TrimSpace(s)}, true
			}
		}
		return Value{Raw: ""}, true
	// *Conn / *Reader / *Writer methods
	case "Close", "PrintfLine", "ReadLine", "ReadLineBytes",
		"ReadContinuedLine", "ReadContinuedLineBytes",
		"ReadMIMEHeader", "ReadDotLines", "ReadDotBytes":
		return Value{}, true
	case "ReadResponse":
		return Value{Raw: []Value{{Raw: int64(200)}, {Raw: ""}, {}}}, true
	}
	return Value{}, true
}

// handleGoTokenCall models go/token.* functions (#163).
func (interp *Interpreter) handleGoTokenCall(gid int64, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewFileSet":
		return opaque, true
	case "NewFile":
		return opaque, true
	// FileSet methods
	case "AddFile":
		return opaque, true
	case "File":
		return opaque, true
	case "Base", "Size":
		return Value{Raw: int64(1)}, true
	case "Iterate":
		// Calls callback with each *File — probe callback if provided.
		if len(args) >= 2 {
			probe := func(argIdx int, callArgs []Value) {
				if argIdx >= len(args) {
					return
				}
				switch fn := args[argIdx].Raw.(type) {
				case *ssa.Function:
					if fn.Blocks != nil {
						interp.execFunction(gid, fn, callArgs)
					}
				case *ClosureValue:
					all := append(callArgs, fn.FreeVars...)
					interp.execFunction(gid, fn.Fn, all)
				}
			}
			probe(1, []Value{opaque})
		}
		return Value{}, true
	// File methods
	case "Pos", "End", "LineCount":
		return Value{Raw: int64(1)}, true
	case "Name":
		return Value{Raw: "<go/token.File>"}, true
	case "SetLinesForContent":
		return Value{}, true
	case "Line", "Offset", "Position":
		return opaque, true
	// Pos methods (Pos is just int32, but method calls on token.Pos)
	case "IsValid":
		return Value{Raw: true}, true
	// Token methods
	case "String":
		return Value{Raw: "<token>"}, true
	case "IsLiteral", "IsOperator", "IsKeyword":
		return Value{Raw: false}, true
	case "Lookup":
		// token.Lookup(ident string) Token — returns IDENT (5).
		return Value{Raw: int64(5)}, true
	}
	return Value{}, true
}

// handleGoASTCall models go/ast.* functions (#163).
// gid/site are needed for callback probing in ast.Inspect/ast.Walk.
func (interp *Interpreter) handleGoASTCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	probe := func(argIdx int, callArgs []Value) {
		if argIdx >= len(args) {
			return
		}
		switch fn := args[argIdx].Raw.(type) {
		case *ssa.Function:
			if fn.Blocks != nil {
				interp.execFunction(gid, fn, callArgs)
			}
		case *ClosureValue:
			all := append(callArgs, fn.FreeVars...)
			interp.execFunction(gid, fn.Fn, all)
		}
	}
	switch name {
	case "Inspect":
		// ast.Inspect(node Node, f func(Node) bool) — probe f with opaque node.
		probe(1, []Value{opaque})
		return Value{}, true
	case "Walk":
		// ast.Walk(v Visitor, node Node) — probe Visit with opaque node.
		probe(0, []Value{opaque})
		return Value{}, true
	case "Print":
		return Value{}, true
	case "Fprint":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "NewIdent":
		return opaque, true
	case "NewScope":
		return opaque, true
	case "IsExported":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z'}, true
			}
		}
		return Value{Raw: false}, true
	case "FileExports", "FilterDecl", "FilterFile", "FilterPackage":
		return Value{Raw: false}, true
	case "PackageExports":
		return Value{Raw: false}, true
	case "SortImports":
		return Value{}, true
	case "MergePackageFiles":
		return opaque, true
	}
	return Value{}, true
}

// handleGoParserCall models go/parser.* functions (#163).
func (interp *Interpreter) handleGoParserCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ParseFile":
		// Returns (*ast.File, error) — opaque file, nil error.
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParseDir":
		// Returns (map[string]*ast.Package, error) — empty map, nil error.
		return Value{Raw: []Value{{Raw: map[interface{}]Value{}}, {}}}, true
	case "ParseExpr":
		// Returns (ast.Expr, error) — opaque expr, nil error.
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParseExprFrom":
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, true
}

// handleGoFormatCall models go/format.* functions (#163).
func (interp *Interpreter) handleGoFormatCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Source":
		// Returns ([]byte, error) — empty byte slice, nil error.
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Node":
		// Returns ([]byte, error) — empty byte slice, nil error.
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, true
}

// handleSyscallCall models syscall.* functions (#164).
// These are low-level OS calls; return safe concrete values where possible.
func (interp *Interpreter) handleSyscallCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Getpid", "Getppid":
		return Value{Raw: int64(os.Getpid())}, true
	case "Getuid", "Geteuid":
		return Value{Raw: int64(os.Getuid())}, true
	case "Getgid", "Getegid":
		return Value{Raw: int64(os.Getgid())}, true
	case "Getenv":
		// syscall.Getenv returns (string, bool) unlike os.Getenv.
		val := ""
		found := false
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				val = os.Getenv(s)
				found = val != ""
			}
		}
		return Value{Raw: []Value{{Raw: val}, {Raw: found}}}, true
	case "Getcwd":
		wd, _ := os.Getwd()
		return Value{Raw: []Value{{Raw: wd}, {}}}, true
	case "Open", "OpenAt":
		// Returns (fd int, err error) — opaque fd, nil error.
		return Value{Raw: []Value{{Raw: int64(3)}, {}}}, true
	case "Close":
		return Value{}, true // nil error
	case "Read", "Write", "Pread", "Pwrite":
		// Returns (n int, err error).
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Stat", "Lstat", "Fstat":
		// Returns (Stat_t, error) — opaque stat, nil error.
		return Value{Raw: []Value{opaque, {}}}, true
	case "Mkdir", "MkdirAll", "Remove", "RemoveAll", "Rename", "Symlink", "Link":
		return Value{}, true // nil error
	case "Chdir", "Chmod", "Chown", "Lchown":
		return Value{}, true // nil error
	case "Kill":
		return Value{}, true // nil error
	case "Getpagesize":
		return Value{Raw: int64(4096)}, true
	case "Exit":
		for _, g := range interp.goroutines {
			g.Panicked = true
		}
		return Value{}, true
	case "Mmap":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Munmap":
		return Value{}, true // nil error
	case "ForkExec", "StartProcess":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Wait4", "WaitPid":
		return Value{Raw: []Value{opaque, opaque, {}}}, true
	case "Sysctl", "SysctlUint32":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "Socket", "Bind", "Connect", "Listen", "Accept",
		"Shutdown", "Setsockopt", "Getsockopt":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Sendto", "Recvfrom":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Pipe":
		return Value{}, true // nil error
	case "Dup", "Dup2":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Seek", "Fsync", "Ftruncate", "Fallocate":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Setenv", "Unsetenv", "Clearenv":
		return Value{}, true // nil error
	case "Umask":
		return Value{Raw: int64(0022)}, true
	case "ENOENT", "EACCES", "EPERM", "EEXIST", "EINVAL":
		// Errno constants returned as int64.
		return Value{Raw: int64(0)}, true
	case "StringByteSlice", "StringBytePtr", "StringSlicePtrGroups":
		return Value{Raw: []Value{}}, true
	}
	return Value{}, true
}

// handleTestingIotestCall models testing/iotest.* functions (#164).
func (interp *Interpreter) handleTestingIotestCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ErrReader":
		return opaque, true
	case "NewReadLogger", "NewWriteLogger":
		return opaque, true
	case "OneByteReader", "HalfReader", "DataErrReader", "TimeoutReader":
		return opaque, true
	case "TruncateWriter":
		return opaque, true
	case "TestReader":
		// Returns error — nil (success).
		return Value{}, true
	}
	return Value{}, true
}

// handleTestingFstestCall models testing/fstest.* functions (#164).
func (interp *Interpreter) handleTestingFstestCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "TestFS":
		// Returns error — nil (success).
		return Value{}, true
	// MapFS is just a map — creation and operations are handled by ssa.MakeMap.
	// Method calls on MapFS:
	case "Open":
		return Value{Raw: []Value{opaque, {}}}, true
	case "ReadDir":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ReadFile":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Stat":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Sub":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Glob":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, true
}

// handleHTTPTestCall models net/http/httptest.* functions (#165).
func (interp *Interpreter) handleHTTPTestCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewRecorder":
		// Returns *ResponseRecorder.
		return opaque, true
	case "NewServer", "NewTLSServer", "NewUnstartedServer":
		// Returns *Server.
		return opaque, true
	// *ResponseRecorder methods.
	case "Code":
		return Value{Raw: int64(200)}, true
	case "Header":
		// Returns http.Header (a map).
		return Value{Raw: map[interface{}]Value{}}, true
	case "Body":
		return opaque, true
	case "Result":
		return opaque, true
	case "Flush":
		return Value{}, true
	case "WriteHeader":
		return Value{}, true
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "WriteString":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	// *Server methods.
	case "Close", "CloseClientConnections":
		return Value{}, true
	case "Start", "StartTLS":
		return Value{}, true
	case "URL", "Listener", "Config", "Certificate":
		return opaque, true
	case "Client":
		return opaque, true
	}
	return Value{}, true
}

// handleHTTPUtilCall models net/http/httputil.* functions (#165).
func (interp *Interpreter) handleHTTPUtilCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReverseProxy", "NewSingleHostReverseProxy":
		return opaque, true
	case "DumpRequest", "DumpRequestOut":
		// Returns ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "DumpResponse":
		// Returns ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "NewChunkedReader", "NewChunkedWriter":
		return opaque, true
	// *ReverseProxy methods.
	case "ServeHTTP":
		return Value{}, true
	case "ModifyResponse":
		return Value{}, true
	// BufferPool interface — opaque.
	case "Get", "Put":
		return Value{Raw: []byte(nil)}, true
	}
	return Value{}, true
}

// handleNetRPCCall models net/rpc.* functions (#166).
func (interp *Interpreter) handleNetRPCCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Dial", "DialHTTP", "DialHTTPPath":
		// Returns (*Client, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewClient", "NewClientWithCodec":
		return opaque, true
	// *Client methods.
	case "Call":
		// Returns error.
		return Value{}, true
	case "Go":
		// Returns *Call.
		return opaque, true
	case "Close":
		return Value{}, true
	// Server-side.
	case "Register", "RegisterName":
		return Value{}, true
	case "ServeConn", "Accept", "HandleHTTP", "ServeHTTP":
		return Value{}, true
	case "NewServer":
		return opaque, true
	}
	return Value{}, true
}

// handleDebugPprofCall models debug/pprof.* functions (#167).
func (interp *Interpreter) handleDebugPprofCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Lookup":
		// Returns *Profile (opaque, non-nil).
		return opaque, true
	case "Profiles":
		// Returns []*Profile.
		return Value{Raw: []Value{opaque}}, true
	case "NewProfile":
		return opaque, true
	case "Handler":
		// Returns http.Handler.
		return opaque, true
	// *Profile methods.
	case "Name":
		return Value{Raw: "<pprof.Profile>"}, true
	case "Count":
		return Value{Raw: int64(0)}, true
	case "Add":
		return Value{}, true
	case "Remove":
		return Value{}, true
	case "WriteTo":
		// (error)
		return Value{}, true
	}
	return Value{}, true
}

// handleNetHTTPPprofCall models net/http/pprof.* functions (#167).
// All functions are http.HandlerFunc wrappers — model as noops.
func (interp *Interpreter) handleNetHTTPPprofCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Index", "Cmdline", "Profile", "Symbol", "Trace", "Handler":
		return Value{}, true
	}
	return Value{}, true
}

// handlePluginCall models plugin.* functions (#168).
func (interp *Interpreter) handlePluginCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Open":
		// Returns (*Plugin, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// *Plugin methods.
	case "Lookup":
		// Returns (Symbol, error) — Symbol is interface{}.
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, true
}

// handleSemaphoreCall models golang.org/x/sync/semaphore.* functions (#168).
func (interp *Interpreter) handleSemaphoreCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewWeighted":
		// Returns *Weighted.
		return opaque, true
	// *Weighted methods.
	case "Acquire":
		// Returns error — model as success (nil), not blocking.
		return Value{}, true
	case "Release":
		return Value{}, true
	case "TryAcquire":
		// Returns bool — true (acquired successfully).
		return Value{Raw: true}, true
	}
	return Value{}, true
}

// ── v0.57.0 new handlers (#169-172) ──────────────────────────────────────────

// handleIOIoutilCall models io/ioutil.* functions (#169).
// io/ioutil is deprecated since Go 1.16 but widely used in older code.
func (interp *Interpreter) handleIOIoutilCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ReadAll":
		// Returns ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ReadFile":
		// Returns ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "WriteFile":
		// Returns error.
		return Value{}, true
	case "TempFile":
		// Returns (*os.File, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "TempDir":
		// Returns (string, error).
		return Value{Raw: []Value{{Raw: "/tmp/giri-tmpdir"}, {}}}, true
	case "NopCloser":
		// Returns io.ReadCloser (opaque).
		return opaque, true
	case "ReadDir":
		// Returns ([]os.FileInfo, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	// Discard is a variable (not a function), handled via globals.
	return Value{}, true
}

// handleCompressBzip2Call models compress/bzip2.* functions (#170).
func (interp *Interpreter) handleCompressBzip2Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader":
		// Returns io.Reader.
		return opaque, true
	}
	return Value{}, true
}

// handleCompressFlateCall models compress/flate.* functions (#170).
func (interp *Interpreter) handleCompressFlateCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader", "NewReaderDict":
		// Returns (io.ReadCloser, error) for NewReaderDict; io.ReadCloser for NewReader.
		if name == "NewReaderDict" {
			return Value{Raw: []Value{opaque, {}}}, true
		}
		return opaque, true
	case "NewWriter", "NewWriterDict":
		// Returns (*Writer, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// *Reader / *Writer / *Resetter methods.
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Close":
		return Value{}, true
	case "Reset":
		return Value{}, true
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Flush":
		return Value{}, true
	}
	return Value{}, true
}

// handleCompressLZWCall models compress/lzw.* functions (#170).
func (interp *Interpreter) handleCompressLZWCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader":
		// Returns io.ReadCloser.
		return opaque, true
	case "NewWriter":
		// Returns io.WriteCloser.
		return opaque, true
	// io.ReadCloser / io.WriteCloser methods.
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Close":
		return Value{}, true
	}
	return Value{}, true
}

// handleGoTypesCall models go/types.* functions (#171).
func (interp *Interpreter) handleGoTypesCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors.
	case "NewPackage":
		return opaque, true
	case "NewScope":
		return opaque, true
	case "NewNamed":
		return opaque, true
	case "NewStruct":
		return opaque, true
	case "NewInterface", "NewInterfaceType":
		return opaque, true
	case "NewSignature", "NewSignatureType":
		return opaque, true
	case "NewFunc":
		return opaque, true
	case "NewVar":
		return opaque, true
	case "NewConst":
		return opaque, true
	case "NewTypeName":
		return opaque, true
	case "NewLabel":
		return opaque, true
	case "NewPkgName":
		return opaque, true
	case "NewArray":
		return opaque, true
	case "NewSlice":
		return opaque, true
	case "NewMap":
		return opaque, true
	case "NewChan":
		return opaque, true
	case "NewPointer":
		return opaque, true
	case "NewTuple":
		return opaque, true
	case "NewParam":
		return opaque, true
	// Checker.
	case "NewChecker":
		return opaque, true
	// Config.
	case "Check":
		// (*Config).Check returns (*Package, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// Predicate functions.
	case "Implements":
		return Value{Raw: false}, true
	case "AssignableTo":
		return Value{Raw: false}, true
	case "ConvertibleTo":
		return Value{Raw: false}, true
	case "Identical":
		return Value{Raw: false}, true
	case "IsInterface":
		return Value{Raw: false}, true
	case "TypeString":
		return Value{Raw: "<type>"}, true
	case "ObjectString":
		return Value{Raw: "<object>"}, true
	case "SelectionString":
		return Value{Raw: "<selection>"}, true
	case "WriteType", "WriteSignature":
		return Value{}, true
	case "Eval":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Universe":
		return opaque, true
	case "NewMethodSet":
		return opaque, true
	case "LookupFieldOrMethod":
		return Value{Raw: []Value{opaque, {Raw: []Value{}}, {Raw: false}}}, true
	case "MissingMethod":
		return Value{Raw: []Value{opaque, {Raw: false}}}, true
	case "Default":
		return opaque, true
	case "Unalias":
		return opaque, true
	// Scope methods.
	case "Lookup":
		return opaque, true
	case "Insert":
		return opaque, true
	case "Names":
		return Value{Raw: []Value{}}, true
	case "Len":
		return Value{Raw: int64(0)}, true
	case "Parent":
		return opaque, true
	case "Contains":
		return Value{Raw: false}, true
	// Named type methods.
	case "AddMethod":
		return Value{}, true
	case "SetUnderlying":
		return Value{}, true
	case "Underlying":
		return opaque, true
	case "NumMethods":
		return Value{Raw: int64(0)}, true
	case "Method":
		return opaque, true
	case "Obj":
		return opaque, true
	case "String":
		return Value{Raw: "<types.Type>"}, true
	// Object methods.
	case "Name":
		return Value{Raw: "<name>"}, true
	case "Pkg":
		return opaque, true
	case "Type":
		return opaque, true
	case "Exported":
		return Value{Raw: false}, true
	case "Id":
		return Value{Raw: "<id>"}, true
	case "Pos":
		return Value{Raw: int64(0)}, true
	// Interface methods.
	case "Complete":
		return opaque, true
	case "NumEmbeddeds", "NumExplicitMethods":
		return Value{Raw: int64(0)}, true
	// Struct methods.
	case "NumFields":
		return Value{Raw: int64(0)}, true
	case "Field":
		return opaque, true
	case "Tag":
		return Value{Raw: ""}, true
	// Signature methods.
	case "Params", "Results":
		return opaque, true
	case "Recv":
		return opaque, true
	case "Variadic":
		return Value{Raw: false}, true
	}
	return Value{}, true
}

// handleGoImporterCall models go/importer.* functions (#171).
func (interp *Interpreter) handleGoImporterCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Default":
		return opaque, true
	case "For":
		return opaque, true
	case "ForCompiler":
		return opaque, true
	// Importer interface method.
	case "Import":
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, true
}

// handleGoBuildCall models go/build.* functions (#171).
func (interp *Interpreter) handleGoBuildCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Default":
		// build.Default is a Context variable, not a function;
		// method calls on it come through as name == method name.
		return opaque, true
	case "Import":
		return Value{Raw: []Value{opaque, {}}}, true
	case "ImportDir":
		return Value{Raw: []Value{opaque, {}}}, true
	case "IsDir":
		return Value{Raw: false}, true
	case "IsLocalImport":
		return Value{Raw: false}, true
	case "ArchChar":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	// (*Context) methods.
	case "MatchFile":
		return Value{Raw: []Value{{Raw: false}, {}}}, true
	case "SrcDirs":
		return Value{Raw: []Value{}}, true
	}
	return Value{}, true
}

// handleGoDocCall models go/doc.* functions (#171).
func (interp *Interpreter) handleGoDocCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New", "NewFromFiles":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Examples":
		return Value{Raw: []Value{}}, true
	case "Synopsis":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				// Return first sentence-ish.
				if len(s) > 80 {
					s = s[:80]
				}
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	case "ToHTML", "ToText", "ToMarkdown":
		return Value{}, true
	case "IsPredeclared":
		return Value{Raw: false}, true
	}
	return Value{}, true
}

// handleCookiejarCall models net/http/cookiejar.* functions (#172).
func (interp *Interpreter) handleCookiejarCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		// Returns (*Jar, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// http.CookieJar interface methods.
	case "SetCookies":
		return Value{}, true
	case "Cookies":
		// Returns []*http.Cookie.
		return Value{Raw: []Value{}}, true
	}
	return Value{}, true
}

// ── v0.58.0 new handlers (#173-176) ──────────────────────────────────────────

// handleCryptoSubtleCall models crypto/subtle.* functions (#173).
func (interp *Interpreter) handleCryptoSubtleCall(name string, args []Value) (Value, bool) {
	switch name {
	case "ConstantTimeCompare":
		// ([]byte, []byte) → int (1 if equal, 0 otherwise).
		return Value{Raw: int64(0)}, true
	case "ConstantTimeByteEq":
		return Value{Raw: int64(0)}, true
	case "ConstantTimeLessOrEq":
		return Value{Raw: int64(0)}, true
	case "ConstantTimeSelect":
		// (v, x, y int) → int: returns x if v==1, else y.
		return Value{Raw: int64(0)}, true
	case "ConstantTimeCopy":
		// (v int, x, y []byte): noop.
		return Value{}, true
	case "XORBytes":
		// (dst, x, y []byte) → int.
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: int64(n)}, true
	case "WithDataIndependentTiming":
		// (f func()) — call f.
		if len(args) > 0 {
			switch fn := args[0].Raw.(type) {
			case *ssa.Function:
				if fn.Blocks != nil {
					interp.execFunction(0, fn, nil)
				}
			case *ClosureValue:
				interp.execFunction(0, fn.Fn, append([]Value{}, fn.FreeVars...))
			}
		}
		return Value{}, true
	}
	return Value{}, true
}

// handleMapHashCall models hash/maphash.* functions (#173).
func (interp *Interpreter) handleMapHashCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Package-level funcs (Go 1.19+).
	case "Bytes":
		return Value{Raw: uint64(0)}, true
	case "String":
		return Value{Raw: uint64(0)}, true
	case "MakeSeed":
		return opaque, true
	// *Hash methods.
	case "New":
		return opaque, true
	case "Write", "WriteByte", "WriteString":
		n := 0
		if len(args) > 1 {
			switch v := args[1].Raw.(type) {
			case []Value:
				n = len(v)
			case string:
				n = len(v)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Sum64":
		return Value{Raw: uint64(0)}, true
	case "Sum32":
		return Value{Raw: uint32(0)}, true
	case "Sum":
		return Value{Raw: []Value{}}, true
	case "Reset":
		return Value{}, true
	case "SetSeed":
		return Value{}, true
	case "Seed":
		return opaque, true
	case "BlockSize", "Size":
		return Value{Raw: int64(8)}, true
	}
	return Value{}, true
}

// handleRegexpSyntaxCall models regexp/syntax.* functions (#173).
func (interp *Interpreter) handleRegexpSyntaxCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Parse":
		// Returns (*Regexp, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "MustParse":
		return opaque, true
	case "Compile":
		// Returns (*Prog, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "SimplifyRegexp":
		return opaque, true
	case "IsWordChar":
		return Value{Raw: false}, true
	case "Flags":
		return Value{Raw: int64(0)}, true
	// *Regexp / *Prog methods.
	case "String":
		return Value{Raw: "<regexp>"}, true
	case "Equal":
		return Value{Raw: false}, true
	case "Simplify":
		return opaque, true
	case "CapNames":
		return Value{Raw: []Value{}}, true
	case "MaxCap":
		return Value{Raw: int64(0)}, true
	case "Prefix":
		return Value{Raw: []Value{{Raw: ""}, {Raw: false}}}, true
	}
	return Value{}, true
}

// handleUniqueCall models unique.* functions (#173).
// unique.Make is a generic function (Go 1.23+).
func (interp *Interpreter) handleUniqueCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Make":
		// Make[T comparable](v T) Handle[T] — return opaque Handle.
		return opaque, true
	// Handle[T] methods.
	case "Value":
		// Returns the stored T — return zero Value.
		if len(args) > 0 {
			return args[0], true
		}
		return Value{}, true
	}
	return Value{}, true
}

// handleGoPrinterCall models go/printer.* functions (#174).
func (interp *Interpreter) handleGoPrinterCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Fprint", "Fprintf":
		// (io.Writer, *token.FileSet, node interface{}) (int, error).
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Sprint":
		// (*token.FileSet, node interface{}) (string, error).
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	}
	return Value{}, true
}

// handleGoConstantCall models go/constant.* functions (#174).
func (interp *Interpreter) handleGoConstantCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors.
	case "MakeInt64":
		if len(args) > 0 {
			return Value{Raw: toInt64(args[0])}, true
		}
		return Value{Raw: int64(0)}, true
	case "MakeUint64":
		if len(args) > 0 {
			return Value{Raw: toInt64(args[0])}, true
		}
		return Value{Raw: int64(0)}, true
	case "MakeFloat64":
		if len(args) > 0 {
			if f, ok := args[0].Raw.(float64); ok {
				return Value{Raw: f}, true
			}
		}
		return Value{Raw: float64(0)}, true
	case "MakeString":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: ""}, true
	case "MakeBool":
		if len(args) > 0 {
			if b, ok := args[0].Raw.(bool); ok {
				return Value{Raw: b}, true
			}
		}
		return Value{Raw: false}, true
	case "MakeFromLiteral":
		// (lit string, tok token.Token, zero uint) → Value.
		// Return a string representation as opaque.
		return opaque, true
	case "MakeImag":
		return opaque, true
	// Operations.
	case "BinaryOp":
		return opaque, true
	case "UnaryOp":
		return opaque, true
	case "Compare":
		return Value{Raw: false}, true
	case "Shift":
		return opaque, true
	case "ToComplex":
		return opaque, true
	case "Real", "Imag":
		return opaque, true
	// Extractors.
	case "StringVal":
		return Value{Raw: ""}, true
	case "Int64Val":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: true}}}, true
	case "Uint64Val":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: true}}}, true
	case "Float64Val":
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: true}}}, true
	case "Float32Val":
		return Value{Raw: []Value{{Raw: float64(0)}, {Raw: true}}}, true
	case "BoolVal":
		return Value{Raw: false}, true
	// Value methods.
	case "Kind":
		return Value{Raw: int64(0)}, true
	case "Sign":
		return Value{Raw: int64(0)}, true
	case "BitLen":
		return Value{Raw: int64(0)}, true
	case "String":
		return Value{Raw: "<constant>"}, true
	case "ExactString":
		return Value{Raw: "<constant>"}, true
	}
	return Value{}, true
}

// handleGoScannerCall models go/scanner.* functions (#174).
func (interp *Interpreter) handleGoScannerCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Scanner methods (receiver = args[0]).
	case "Init":
		return Value{}, true
	case "Scan":
		// Returns (token.Pos, token.Token, string).
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {Raw: ""}}}, true
	case "Peek":
		return Value{Raw: int64(0)}, true
	// ErrorList.
	case "Add":
		return Value{}, true
	case "Sort":
		return Value{}, true
	case "Error":
		return Value{Raw: ""}, true
	case "Err":
		return Value{}, true
	case "Reset":
		return Value{}, true
	case "Len":
		return Value{Raw: int64(0)}, true
	// PrintError.
	case "PrintError":
		return Value{}, true
	case "Mode":
		return opaque, true
	}
	return Value{}, true
}

// handleGoVersionCall models go/version.* functions (#174).
// Available since Go 1.22.
func (interp *Interpreter) handleGoVersionCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Compare":
		// (x, y string) → int.
		if len(args) >= 2 {
			sx, sxok := args[0].Raw.(string)
			sy, syok := args[1].Raw.(string)
			if sxok && syok {
				if sx < sy {
					return Value{Raw: int64(-1)}, true
				} else if sx > sy {
					return Value{Raw: int64(1)}, true
				}
				return Value{Raw: int64(0)}, true
			}
		}
		return Value{Raw: int64(0)}, true
	case "IsValid":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: len(s) >= 2 && s[0] == 'g' && s[1] == 'o'}, true
		}
		return Value{Raw: false}, true
	case "Lang":
		// ("go1.21.4") → "go1.21".
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: s}, true
		}
		return Value{Raw: ""}, true
	case "Max":
		if len(args) >= 2 {
			sx, sxok := args[0].Raw.(string)
			sy, syok := args[1].Raw.(string)
			if sxok && syok {
				if sx >= sy {
					return Value{Raw: sx}, true
				}
				return Value{Raw: sy}, true
			}
		}
		return Value{Raw: ""}, true
	}
	return Value{}, true
}

// handleDebugBuildinfoCall models debug/buildinfo.* functions (#175).
func (interp *Interpreter) handleDebugBuildinfoCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ReadFile":
		// Returns (*BuildInfo, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "Read":
		// Returns (*BuildInfo, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// BuildInfo methods.
	case "String":
		return Value{Raw: "<buildinfo>"}, true
	case "MarshalText":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "UnmarshalText":
		return Value{}, true
	}
	return Value{}, true
}

// handleDebugDWARFCall models debug/dwarf.* functions (#175).
func (interp *Interpreter) handleDebugDWARFCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		return Value{Raw: []Value{opaque, {}}}, true
	// *Data methods.
	case "Reader":
		return opaque, true
	case "Type":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Ranges":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "LineReader":
		return Value{Raw: []Value{opaque, {}}}, true
	// *Reader methods.
	case "Next":
		return Value{Raw: []Value{opaque, {}}}, true
	case "SeekPC":
		return Value{Raw: []Value{opaque, {}}}, true
	case "SkipChildren":
		return Value{}, true
	case "Seek":
		return Value{}, true
	case "AddressSize":
		return Value{Raw: int64(8)}, true
	case "ByteOrder":
		return opaque, true
	// *LineReader methods.
	case "Tell":
		return Value{}, true
	// *Entry methods.
	case "Val":
		return opaque, true
	case "AttrField":
		return opaque, true
	}
	return Value{}, true
}

// handleDebugELFCall models debug/elf.* functions (#175).
func (interp *Interpreter) handleDebugELFCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Open":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewFile":
		return Value{Raw: []Value{opaque, {}}}, true
	// *File methods.
	case "Close":
		return Value{}, true
	case "Section":
		return opaque, true
	case "SectionByType":
		return opaque, true
	case "Symbols":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "DynamicSymbols":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ImportedSymbols":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ImportedLibraries":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "DWARF":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Segments":
		return Value{Raw: []Value{}}, true
	case "Sections":
		return Value{Raw: []Value{}}, true
	// Section methods (Open handled above — returns opaque io.ReadSeeker for sections too).
	case "Data":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ReadAt":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	// ST_* constants etc.
	case "R_INFO", "R_SYM", "R_TYPE", "ST_INFO", "ST_BIND", "ST_TYPE", "ST_VISIBILITY":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, true
}

// handleDebugMachoCall models debug/macho.* functions (#175).
func (interp *Interpreter) handleDebugMachoCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Open":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewFile":
		return Value{Raw: []Value{opaque, {}}}, true
	// *File methods.
	case "Close":
		return Value{}, true
	case "Section":
		return opaque, true
	case "Segment":
		return opaque, true
	case "Symtab":
		return opaque, true
	case "Dysymtab":
		return opaque, true
	case "DWARF":
		return Value{Raw: []Value{opaque, {}}}, true
	case "ImportedSymbols":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ImportedLibraries":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	// Section methods (Open shared with file-level Open above).
	case "Data":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ReadAt":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	}
	return Value{}, true
}

// handleDebugPECall models debug/pe.* functions (#175).
func (interp *Interpreter) handleDebugPECall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Open":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewFile":
		return Value{Raw: []Value{opaque, {}}}, true
	// *File methods.
	case "Close":
		return Value{}, true
	case "Section":
		return opaque, true
	case "DWARF":
		return Value{Raw: []Value{opaque, {}}}, true
	case "ImportedSymbols":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ImportedLibraries":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	// Section methods (Open shared with file-level Open above).
	case "Data":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ReadAt":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	}
	return Value{}, true
}

// handleTestingQuickCall models testing/quick.* functions (#176).
func (interp *Interpreter) handleTestingQuickCall(gid int64, site, name string, args []Value) (Value, bool) {
	switch name {
	case "Check", "CheckCustom":
		// Check(f interface{}, rand *rand.Rand) error — probe f with zero args.
		if len(args) > 0 {
			switch fn := args[0].Raw.(type) {
			case *ssa.Function:
				if fn.Blocks != nil {
					// Build zero-value args for each param.
					zeroArgs := make([]Value, len(fn.Params))
					interp.execFunction(gid, fn, zeroArgs)
				}
			case *ClosureValue:
				zeroArgs := make([]Value, len(fn.Fn.Params))
				all := append(zeroArgs, fn.FreeVars...)
				interp.execFunction(gid, fn.Fn, all)
			}
		}
		// Return nil error (test passes).
		return Value{}, true
	case "Value":
		// Value(t reflect.Type, rand *rand.Rand) (reflect.Value, bool).
		return Value{Raw: []Value{{Raw: struct{}{}}, {Raw: false}}}, true
	}
	return Value{}, true
}

// handleQuotedPrintableCall models mime/quotedprintable.* functions (#176).
func (interp *Interpreter) handleQuotedPrintableCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewReader":
		return opaque, true
	case "NewWriter":
		return opaque, true
	// *Reader methods.
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	// *Writer methods.
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Close":
		return Value{}, true
	}
	return Value{}, true
}

// handleHTTPTraceCall models net/http/httptrace.* functions (#176).
func (interp *Interpreter) handleHTTPTraceCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "WithClientTrace":
		// (ctx context.Context, trace *ClientTrace) → context.Context.
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true
	case "ContextClientTrace":
		// (ctx context.Context) → *ClientTrace (opaque).
		return opaque, true
	}
	return Value{}, true
}

// handleJSONRPCCall models net/rpc/jsonrpc.* functions (#176).
func (interp *Interpreter) handleJSONRPCCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewClient":
		// Returns *rpc.Client.
		return opaque, true
	case "Dial":
		// Returns (*rpc.Client, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewServerCodec":
		// Returns rpc.ServerCodec.
		return opaque, true
	case "NewClientCodec":
		// Returns rpc.ClientCodec.
		return opaque, true
	case "ServeConn":
		return Value{}, true
	}
	return Value{}, true
}

// ── v0.59.0 new handlers (#177-180) ──────────────────────────────────────────

// handleGoBuildConstraintCall models go/build/constraint.* functions (#177).
func (interp *Interpreter) handleGoBuildConstraintCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "IsGoBuild", "IsPlusBuild":
		if s, ok := stdlibArgString(args, 0); ok {
			return Value{Raw: len(s) > 0}, true
		}
		return Value{Raw: false}, true
	case "GoVersion":
		// GoVersion(x Expr) string.
		return Value{Raw: "go1.21"}, true
	case "Parse", "ParseLine":
		// Returns (Expr, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "Check":
		// Returns bool.
		return Value{Raw: true}, true
	case "Eval":
		// (x Expr, ok func(tag string) bool) bool.
		return Value{Raw: true}, true
	case "PlusBuildLines":
		// Returns ([]string, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "AndExpr", "OrExpr", "NotExpr", "TagExpr", "IsSatisfied":
		return opaque, true
	// Expr.String — shares "String" case above (no separate entry needed).
	}
	return Value{}, true
}

// handleGoDocCommentCall models go/doc/comment.* functions (#177).
func (interp *Interpreter) handleGoDocCommentCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// (*Parser).Parse.
	case "Parse":
		return opaque, true
	// (*Printer) output methods.
	case "HTML", "Markdown", "Text", "Comment":
		return Value{Raw: []Value{}}, true
	// DefaultLookupPackage.
	case "DefaultLookupPackage":
		return Value{Raw: []Value{{Raw: ""}, {Raw: false}}}, true
	}
	return Value{}, true
}

// handleTemplateParseCall models text/template/parse.* functions (#177).
func (interp *Interpreter) handleTemplateParseCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		return opaque, true
	case "Parse":
		// Returns (map[string]*Tree, error).
		return Value{Raw: []Value{{Raw: map[interface{}]Value{}}, {}}}, true
	// *Tree methods.
	case "Copy":
		return opaque, true
	case "ErrorContext":
		return Value{Raw: []Value{{Raw: ""}, {Raw: ""}}}, true
	// Node interface methods.
	case "String", "Type":
		return Value{Raw: ""}, true
	case "Position":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, true
}

// handleDebugGosymCall models debug/gosym.* functions (#178).
func (interp *Interpreter) handleDebugGosymCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewLineTable":
		return opaque, true
	case "NewTable":
		// Returns (*Table, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// *Table methods.
	case "PCToFunc":
		return opaque, true
	case "PCToLine":
		return Value{Raw: []Value{{Raw: ""}, {Raw: int64(0)}, {Raw: ""}}}, true
	case "LineToPC":
		return Value{Raw: []Value{{Raw: uint64(0)}, {}}}, true
	case "LookupFunc":
		return opaque, true
	case "LookupSym":
		return opaque, true
	case "Syms":
		return Value{Raw: []Value{}}, true
	// *LineTable methods share PCToLine/LineToPC cases above.
	}
	return Value{}, true
}

// handleDebugPlan9Call models debug/plan9obj.* functions (#178).
func (interp *Interpreter) handleDebugPlan9Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Open":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewFile":
		return Value{Raw: []Value{opaque, {}}}, true
	// *File methods.
	case "Close":
		return Value{}, true
	case "Symbols":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Section":
		return opaque, true
	case "DWARF":
		return Value{Raw: []Value{opaque, {}}}, true
	// *Section methods.
	case "Data":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, true
}

// handleRuntimeMetricsCall models runtime/metrics.* functions (#178).
func (interp *Interpreter) handleRuntimeMetricsCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "All":
		// Returns []Description.
		return Value{Raw: []Value{}}, true
	case "Read":
		// Read([]Sample) — fills in-place; noop since samples are opaque.
		return Value{}, true
	// Sample and Description are struct types — method calls are opaque.
	case "String":
		return Value{Raw: "<metric>"}, true
	case "Descriptions":
		return Value{Raw: map[interface{}]Value{}}, true
	// Value constructors.
	case "Float64Histogram":
		return opaque, true
	// Kind constants.
	case "KindBad", "KindUint64", "KindFloat64", "KindFloat64Histogram":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, true
}

// handleRuntimeCoverageCall models runtime/coverage.* functions (#178).
func (interp *Interpreter) handleRuntimeCoverageCall(name string, args []Value) (Value, bool) {
	switch name {
	case "ClearCounters":
		return Value{}, true
	case "WriteCounters":
		return Value{}, true
	case "WriteCountersDir":
		return Value{}, true
	case "WriteMetaDir":
		return Value{}, true
	case "WriteMeta":
		return Value{}, true
	}
	return Value{}, true
}

// handleHTTPCGICall models net/http/cgi.* functions (#179).
func (interp *Interpreter) handleHTTPCGICall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Serve":
		return Value{}, true
	case "Request":
		return Value{Raw: []Value{opaque, {}}}, true
	case "RequestFromMap":
		return Value{Raw: []Value{opaque, {}}}, true
	// *Handler.ServeHTTP.
	case "ServeHTTP":
		return Value{}, true
	}
	return Value{}, true
}

// handleHTTPFCGICall models net/http/fcgi.* functions (#179).
func (interp *Interpreter) handleHTTPFCGICall(name string, args []Value) (Value, bool) {
	switch name {
	case "Serve":
		return Value{}, true
	case "ProcessEnv":
		return Value{Raw: map[interface{}]Value{}}, true
	}
	return Value{}, true
}

// handleASCII85Call models encoding/ascii85.* functions (#179).
func (interp *Interpreter) handleASCII85Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Encode":
		// (dst, src []byte) → int.
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: int64(n)}, true
	case "Decode":
		// (dst, src []byte, flush bool) → (ndst, nsrc int, err error).
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {}}}, true
	case "MaxEncodedLen":
		if len(args) > 0 {
			n := toInt64(args[0])
			return Value{Raw: int64(n/4*5 + 5)}, true
		}
		return Value{Raw: int64(0)}, true
	case "NewEncoder":
		return opaque, true
	case "NewDecoder":
		return opaque, true
	// io.WriteCloser / io.Reader methods.
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Close":
		return Value{}, true
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	}
	return Value{}, true
}

// handleSuffixArrayCall models index/suffixarray.* functions (#179).
func (interp *Interpreter) handleSuffixArrayCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		return opaque, true
	// *Index methods.
	case "Bytes":
		return Value{Raw: []Value{}}, true
	case "Lookup":
		// (s []byte, n int) → []int.
		return Value{Raw: []Value{}}, true
	case "FindAllIndex":
		// (r *regexp.Regexp, n int) → [][]int.
		return Value{Raw: []Value{}}, true
	case "Read":
		return Value{}, true
	case "Write":
		return Value{}, true
	case "Len":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, true
}

// handleSyslogCall models log/syslog.* functions (#179).
// syslog is unix-only; return safe values to avoid false positives.
func (interp *Interpreter) handleSyslogCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		// Returns (*Writer, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "Dial":
		// Returns (*Writer, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewLogger":
		// Returns (*log.Logger, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// *Writer methods.
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Close":
		return Value{}, true
	case "Emerg", "Alert", "Crit", "Err", "Warning", "Notice", "Info", "Debug":
		return Value{}, true
	}
	return Value{}, true
}

// handleCryptoDSACall models crypto/dsa.* functions (#180).
func (interp *Interpreter) handleCryptoDSACall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "GenerateParameters":
		// (params *Parameters, rand io.Reader, sizes ParameterSizes) error.
		return Value{}, true
	case "GenerateKey":
		// (priv *PrivateKey, rand io.Reader) error.
		return Value{}, true
	case "Sign":
		// Returns (*big.Int, *big.Int, error).
		return Value{Raw: []Value{opaque, opaque, {}}}, true
	case "Verify":
		return Value{Raw: false}, true
	// L1024N160, L2048N224, L2048N256, L3072N256 constants.
	case "L1024N160", "L2048N224", "L2048N256", "L3072N256":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, true
}

// handleCryptoEllipticCall models crypto/elliptic.* functions (#180).
func (interp *Interpreter) handleCryptoEllipticCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Curve constructors.
	case "P224", "P256", "P384", "P521":
		return opaque, true
	// Operations on curves.
	case "GenerateKey":
		// (curve Curve, rand io.Reader) (priv []byte, x, y *big.Int, err error).
		return Value{Raw: []Value{{Raw: []Value{}}, opaque, opaque, {}}}, true
	case "Marshal":
		// (curve Curve, x, y *big.Int) []byte.
		return Value{Raw: []Value{}}, true
	case "MarshalCompressed":
		return Value{Raw: []Value{}}, true
	case "Unmarshal":
		// (curve Curve, data []byte) (x, y *big.Int).
		return Value{Raw: []Value{opaque, opaque}}, true
	case "UnmarshalCompressed":
		return Value{Raw: []Value{opaque, opaque}}, true
	// Curve interface methods.
	case "Params":
		return opaque, true
	case "IsOnCurve":
		return Value{Raw: false}, true
	case "Add", "Double":
		return Value{Raw: []Value{opaque, opaque}}, true
	case "ScalarMult", "ScalarBaseMult":
		return Value{Raw: []Value{opaque, opaque}}, true
	}
	return Value{}, true
}

// handleHashCRC64Call models hash/crc64.* functions (#180).
func (interp *Interpreter) handleHashCRC64Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "MakeTable":
		return opaque, true
	case "New":
		return opaque, true
	case "Checksum":
		// (data []byte, tab *Table) uint64.
		return Value{Raw: uint64(0)}, true
	case "Update":
		return Value{Raw: uint64(0)}, true
	// hash.Hash64 methods.
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Sum64":
		return Value{Raw: uint64(0)}, true
	case "Sum":
		return Value{Raw: []Value{}}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(8)}, true
	case "BlockSize":
		return Value{Raw: int64(8)}, true
	}
	return Value{}, true
}

// handleBcryptCall models golang.org/x/crypto/bcrypt.* functions (#180).
func (interp *Interpreter) handleBcryptCall(name string, args []Value) (Value, bool) {
	switch name {
	case "GenerateFromPassword":
		// (password []byte, cost int) ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "CompareHashAndPassword":
		// (hashedPassword, password []byte) error.
		return Value{}, true
	case "Cost":
		// (hashedPassword []byte) (int, error).
		return Value{Raw: []Value{{Raw: int64(10)}, {}}}, true
	}
	return Value{}, true
}

// handleNetHTTP2Call models golang.org/x/net/http2.* functions (#180).
func (interp *Interpreter) handleNetHTTP2Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ConfigureServer":
		return Value{}, true
	case "ConfigureTransport":
		return Value{}, true
	case "ConfigureTransports":
		return Value{}, true
	case "NewFramer":
		return opaque, true
	// Framer methods.
	case "ReadFrame":
		return Value{Raw: []Value{opaque, {}}}, true
	case "WriteData":
		return Value{}, true
	case "WriteHeaders":
		return Value{}, true
	case "WritePriority":
		return Value{}, true
	case "WriteRSTStream":
		return Value{}, true
	case "WriteSettings":
		return Value{}, true
	case "WritePing":
		return Value{}, true
	case "WriteGoAway":
		return Value{}, true
	case "WriteWindowUpdate":
		return Value{}, true
	case "WriteContinuation":
		return Value{}, true
	case "WriteDataPadded":
		return Value{}, true
	case "SetMaxReadFrameSize":
		return Value{}, true
	// Server/Transport helpers.
	case "NewServer":
		return opaque, true
	case "VerboseLogs":
		return Value{}, true
	}
	return Value{}, true
}

// ── v0.60.0 new handlers (#181-184) ──────────────────────────────────────────

// handleCryptoDESCall models crypto/des.* functions (#181).
func (interp *Interpreter) handleCryptoDESCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewCipher", "NewTripleDESCipher":
		// Returns (cipher.Block, error).
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, true
}

// handleCryptoRC4Call models crypto/rc4.* functions (#181).
func (interp *Interpreter) handleCryptoRC4Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewCipher":
		// Returns (*Cipher, error).
		return Value{Raw: []Value{opaque, {}}}, true
	// *Cipher methods.
	case "XORKeyStream":
		return Value{}, true
	case "Reset":
		return Value{}, true
	}
	return Value{}, true
}

// handleCryptoPBKDF2Call models crypto/pbkdf2.* functions (#181).
// Generic: Key[Hash hash.Hash](h func() Hash, password string, salt []byte, iter, keyLength int) ([]byte, error).
func (interp *Interpreter) handleCryptoPBKDF2Call(name string, args []Value) (Value, bool) {
	switch name {
	case "Key":
		// Returns ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, true
}

// handleCryptoHKDFCall models crypto/hkdf.* functions (#181).
func (interp *Interpreter) handleCryptoHKDFCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Extract":
		// (h, secret, salt) → ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Expand":
		// (h, prk, info, len) → ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Key":
		// Generic convenience wrapper → ([]byte, error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, true
}

// handleCryptoSHA3Call models crypto/sha3.* functions (#182).
func (interp *Interpreter) handleCryptoSHA3Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors.
	case "New224", "New256", "New384", "New512":
		return opaque, true
	case "NewSHAKE128", "NewSHAKE256", "NewCSHAKE128", "NewCSHAKE256":
		return opaque, true
	// Convenience Sum functions return arrays — model as opaque []byte.
	case "Sum224", "Sum256", "Sum384", "Sum512":
		// Returns fixed-size array value; model as opaque.
		return opaque, true
	case "SumSHAKE128", "SumSHAKE256":
		return Value{Raw: []Value{}}, true
	// *SHA3 hash.Hash methods.
	case "Write":
		n := 0
		if len(args) > 1 {
			if sv, ok := args[1].Raw.([]Value); ok {
				n = len(sv)
			}
		}
		return Value{Raw: []Value{{Raw: int64(n)}, {}}}, true
	case "Sum":
		return Value{Raw: []Value{}}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(32)}, true
	case "BlockSize":
		return Value{Raw: int64(136)}, true
	// *SHAKE XOF methods.
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Clone":
		return opaque, true
	}
	return Value{}, true
}

// handleCryptoHPKECall models crypto/hpke.* functions (#182).
func (interp *Interpreter) handleCryptoHPKECall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// KEM constructors.
	case "DHKEM", "MLKEM768", "MLKEM1024", "MLKEM768P256", "MLKEM768X25519", "MLKEM1024P384":
		return opaque, true
	case "NewKEM":
		return Value{Raw: []Value{opaque, {}}}, true
	// KDF constructors.
	case "HKDFSHA256", "HKDFSHA384", "HKDFSHA512", "SHAKE128", "SHAKE256":
		return opaque, true
	case "NewKDF":
		return Value{Raw: []Value{opaque, {}}}, true
	// AEAD constructors.
	case "AES128GCM", "AES256GCM", "ChaCha20Poly1305", "ExportOnly":
		return opaque, true
	case "NewAEAD":
		return Value{Raw: []Value{opaque, {}}}, true
	// Seal/Open.
	case "Seal":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Open":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, true
}

// handleCryptoMLKEMCall models crypto/mlkem.* functions (#182).
func (interp *Interpreter) handleCryptoMLKEMCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Key generation.
	case "GenerateKey768", "GenerateKey1024":
		// Returns (*DecapsulationKey*, error).
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewDecapsulationKey768", "NewDecapsulationKey1024":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewEncapsulationKey768", "NewEncapsulationKey1024":
		return Value{Raw: []Value{opaque, {}}}, true
	// Key methods.
	case "EncapsulationKey":
		return opaque, true
	case "Encapsulate":
		// (ciphertext []byte, sharedKey []byte, err error).
		return Value{Raw: []Value{{Raw: []Value{}}, {Raw: []Value{}}, {}}}, true
	case "Decapsulate":
		// (sharedKey []byte, err error).
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Bytes":
		return Value{Raw: []Value{}}, true
	}
	return Value{}, true
}

// handleCryptoFIPS140Call models crypto/fips140.* functions (#182).
func (interp *Interpreter) handleCryptoFIPS140Call(name string, args []Value) (Value, bool) {
	switch name {
	case "Enabled":
		return Value{Raw: false}, true
	case "Enforced":
		return Value{Raw: false}, true
	case "Version":
		return Value{Raw: ""}, true
	case "WithoutEnforcement":
		// (f func()) — call f.
		if len(args) > 0 {
			switch fn := args[0].Raw.(type) {
			case *ssa.Function:
				if fn.Blocks != nil {
					interp.execFunction(0, fn, nil)
				}
			case *ClosureValue:
				interp.execFunction(0, fn.Fn, append([]Value{}, fn.FreeVars...))
			}
		}
		return Value{}, true
	}
	return Value{}, true
}

// handleSQLDriverCall models database/sql/driver.* functions (#183).
// The driver package defines only interfaces and a few helper functions/errors.
func (interp *Interpreter) handleSQLDriverCall(name string, args []Value) (Value, bool) {
	switch name {
	case "IsValue":
		// IsValue(v interface{}) bool — true if v is a valid driver.Value.
		return Value{Raw: true}, true
	case "IsScanValue":
		return Value{Raw: true}, true
	case "DefaultParameterConverter":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, true
}

// handleX509PKIXCall models crypto/x509/pkix.* functions (#183).
// pkix contains only struct types; most "calls" are method calls on those structs.
func (interp *Interpreter) handleX509PKIXCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Name methods.
	case "String":
		return Value{Raw: "<pkix.Name>"}, true
	case "ToRDNSequence":
		return opaque, true
	case "FillFromRDNSequence":
		return Value{}, true
	case "AppendPKCS7":
		return Value{Raw: []Value{}}, true
	// RDNSequence.String shares the "String" case above.
	}
	return Value{}, true
}

// handleColorPaletteCall models image/color/palette.* functions (#183).
// palette only has package-level variables Plan9 and WebSafe — no callable functions.
// This handler is a no-op since variables are accessed via globals, not function calls.
func (interp *Interpreter) handleColorPaletteCall(name string, args []Value) (Value, bool) {
	return Value{}, true
}

// handleTZDataCall models time/tzdata.* functions (#183).
// tzdata is an import-side-effect-only package (embeds timezone data).
// It has no exported functions.
func (interp *Interpreter) handleTZDataCall(name string, args []Value) (Value, bool) {
	return Value{}, true
}

// handleStructsCall models structs.* functions (#184).
// structs only contains the HostLayout marker type — no callable functions.
func (interp *Interpreter) handleStructsCall(name string, args []Value) (Value, bool) {
	return Value{}, true
}

// handleWeakCall models weak.* functions (#184).
func (interp *Interpreter) handleWeakCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Make":
		// Make[T any](ptr *T) Pointer[T] — return opaque handle.
		return opaque, true
	// Pointer[T] methods.
	case "Value":
		// Returns *T — conservatively return nil.
		return Value{}, true
	}
	return Value{}, true
}

// handleSlogTestCall models testing/slogtest.* functions (#184).
func (interp *Interpreter) handleSlogTestCall(name string, args []Value) (Value, bool) {
	switch name {
	case "TestHandler":
		// (h slog.Handler, results func() []map[string]any) error — return nil.
		return Value{}, true
	case "Run":
		// (t *testing.T, newHandler func(*testing.T) slog.Handler, ...) — noop.
		return Value{}, true
	}
	return Value{}, true
}

// handleSyncTestCall models testing/synctest.* functions (#184).
func (interp *Interpreter) handleSyncTestCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Test":
		// Test(t *testing.T, f func(*testing.T)) — probe f with opaque t.
		if len(args) > 1 {
			opaque := stdlibOpaque
			switch fn := args[1].Raw.(type) {
			case *ssa.Function:
				if fn.Blocks != nil {
					interp.execFunction(0, fn, []Value{opaque})
				}
			case *ClosureValue:
				all := append([]Value{opaque}, fn.FreeVars...)
				interp.execFunction(0, fn.Fn, all)
			}
		}
		return Value{}, true
	case "Wait":
		return Value{}, true
	}
	return Value{}, true
}

// handleSysUnixCall intercepts golang.org/x/sys/unix calls (issue #185).
// Returns safe zero/opaque values so programs using low-level Unix wrappers
// don't cause false panics or false-positive violations.
func (interp *Interpreter) handleSysUnixCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Process / identity
	case "Getpid":
		return Value{Raw: int64(1)}, true
	case "Getppid":
		return Value{Raw: int64(1)}, true
	case "Getuid", "Geteuid", "Getgid", "Getegid":
		return Value{Raw: int64(0)}, true
	// Environment / working dir
	case "Getenv":
		return Value{Raw: []Value{{Raw: ""}, {Raw: false}}}, true
	case "Setenv", "Unsetenv", "Clearenv":
		return Value{}, true
	case "Getwd":
		return Value{Raw: []Value{{Raw: "/tmp"}, {}}}, true
	case "Chdir", "Chmod", "Chown", "Lchown", "Chroot":
		return Value{}, true
	// File operations — return (fd/n/err) tuples
	case "Open":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Close":
		return Value{}, true
	case "Read", "Write", "Pread", "Pwrite":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Stat", "Lstat", "Fstat":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Mkdir", "MkdirAll", "Mkdirat", "Mkfifo":
		return Value{}, true
	case "Remove", "Unlink", "Rmdir":
		return Value{}, true
	case "Rename", "Symlink", "Link":
		return Value{}, true
	case "Readlink":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "Dup", "Dup2", "Dup3":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Pipe", "Pipe2":
		return Value{}, true
	// Network
	case "Socket":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Connect", "Bind", "Listen":
		return Value{}, true
	case "Accept", "Accept4":
		return Value{Raw: []Value{{Raw: int64(0)}, opaque, {}}}, true
	case "Getsockname", "Getpeername":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Setsockopt", "Getsockopt":
		return Value{}, true
	// Signals / process
	case "Kill":
		return Value{}, true
	case "Getpgrp", "Getpgid":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Setpgid", "Setsid":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	// Memory
	case "Mmap":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Munmap":
		return Value{}, true
	// Byte helpers
	case "ByteSliceFromString":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "BytePtrFromString":
		return Value{Raw: []Value{opaque, {}}}, true
	case "ByteSliceToString":
		return Value{Raw: ""}, true
	case "BytePtrToString":
		return Value{Raw: ""}, true
	// Time
	case "ClockGettime", "Gettimeofday":
		return Value{}, true
	// Misc
	case "Access", "Faccessat":
		return Value{}, true
	case "Truncate", "Ftruncate":
		return Value{}, true
	case "Fsync":
		return Value{}, true
	case "Seek":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Utime", "Utimes", "UtimesNano":
		return Value{}, true
	case "CloseOnExec", "SetNonblock":
		return Value{}, true
	}
	return Value{}, false
}

// handleNetHTMLCall intercepts golang.org/x/net/html calls (issue #186).
func (interp *Interpreter) handleNetHTMLCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Parse":
		// Parse(r io.Reader) (*Node, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParseFragment":
		// ParseFragment(r io.Reader, context *Node) ([]*Node, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ParseWithOptions":
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewTokenizer":
		return opaque, true
	case "Render":
		// Render(w io.Writer, n *Node) error
		return Value{}, true
	case "EscapeString":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	case "UnescapeString":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	// *Tokenizer methods
	case "Next":
		return Value{Raw: int64(0)}, true // ErrorToken = 0
	case "Token":
		return opaque, true
	case "Raw":
		return Value{Raw: []Value{}}, true
	case "Text":
		return Value{Raw: []Value{}}, true
	case "TagName":
		return Value{Raw: []Value{{Raw: []Value{}}, {Raw: false}}}, true
	case "TagAttr":
		return Value{Raw: []Value{{Raw: []Value{}}, {Raw: []Value{}}, {Raw: false}}}, true
	case "SetMaxBuf":
		return Value{}, true
	// *Node methods/fields
	case "FirstChild", "LastChild", "NextSibling", "PrevSibling", "Parent":
		return opaque, true
	case "AppendChild", "InsertBefore", "RemoveChild":
		return Value{}, true
	}
	return Value{}, false
}

// handleNetHTMLCharsetCall intercepts golang.org/x/net/html/charset calls (issue #188).
func (interp *Interpreter) handleNetHTMLCharsetCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "DetermineEncoding":
		// (encoding.Encoding, string, bool)
		return Value{Raw: []Value{opaque, {Raw: "utf-8"}, {Raw: false}}}, true
	case "NewReader":
		return opaque, true
	case "NewReaderLabel":
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, false
}

// handleNetPublicSuffixCall intercepts golang.org/x/net/publicsuffix calls (issue #186).
func (interp *Interpreter) handleNetPublicSuffixCall(name string, args []Value) (Value, bool) {
	switch name {
	case "PublicSuffix":
		// (suffix string, icann bool)
		return Value{Raw: []Value{{Raw: "com"}, {Raw: true}}}, true
	case "EffectiveTLDPlusOne":
		// (result string, err error)
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: []Value{{Raw: s}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: "example.com"}, {}}}, true
	case "List":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleNetIDNACall intercepts golang.org/x/net/idna calls (issue #186).
func (interp *Interpreter) handleNetIDNACall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "ToASCII":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: []Value{{Raw: s}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "ToUnicode":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: []Value{{Raw: s}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "Lookup":
		return opaque, true
	case "New":
		return opaque, true
	// *Profile methods — String only (ToASCII/ToUnicode handled above)
	case "String":
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleNetProxyCall intercepts golang.org/x/net/proxy calls (issue #186).
func (interp *Interpreter) handleNetProxyCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "SOCKS5":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Dial":
		return Value{Raw: []Value{opaque, {}}}, true
	case "FromEnvironment", "FromURL":
		return opaque, true
	case "RegisterDialerType":
		return Value{}, true
	case "NewPerHost":
		return opaque, true
	// *PerHost methods
	case "AddFromString", "AddNetwork", "AddZone", "AddHost":
		return Value{}, true
	case "DialContext":
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, false
}

// handleNetNetUtilCall intercepts golang.org/x/net/netutil calls (issue #186).
func (interp *Interpreter) handleNetNetUtilCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "LimitListener":
		return opaque, true
	}
	return Value{}, false
}

// handleHTTPGutsCall intercepts golang.org/x/net/http/httpguts calls (issue #188).
func (interp *Interpreter) handleHTTPGutsCall(name string, args []Value) (Value, bool) {
	switch name {
	case "HeaderValuesContainsToken":
		return Value{Raw: false}, true
	case "RequestWouldUseHTTP1":
		return Value{Raw: true}, true
	case "IsTokenRune":
		return Value{Raw: false}, true
	case "IsHTTPToken":
		return Value{Raw: false}, true
	case "ValidHeaderFieldName", "ValidHeaderFieldValue",
		"ValidHostHeader", "ValidTrailerHeader":
		return Value{Raw: true}, true
	case "PunycodeHostPort":
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: []Value{{Raw: s}, {}}}, true
			}
		}
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	}
	return Value{}, false
}

// handleModSemverCall intercepts golang.org/x/mod/semver calls (issue #187).
func (interp *Interpreter) handleModSemverCall(name string, args []Value) (Value, bool) {
	strArg := func(i int) string {
		if i < len(args) {
			if s, ok := args[i].Raw.(string); ok {
				return s
			}
		}
		return ""
	}
	switch name {
	case "IsValid":
		v := strArg(0)
		ok := len(v) > 0 && v[0] == 'v'
		return Value{Raw: ok}, true
	case "Canonical":
		return Value{Raw: strArg(0)}, true
	case "Major":
		v := strArg(0)
		// Return "vX" portion
		return Value{Raw: v}, true
	case "MajorMinor":
		return Value{Raw: strArg(0)}, true
	case "Prerelease":
		return Value{Raw: ""}, true
	case "Build":
		return Value{Raw: ""}, true
	case "Compare":
		// returns -1, 0, or 1
		v, w := strArg(0), strArg(1)
		if v < w {
			return Value{Raw: int64(-1)}, true
		} else if v > w {
			return Value{Raw: int64(1)}, true
		}
		return Value{Raw: int64(0)}, true
	case "Max":
		v, w := strArg(0), strArg(1)
		if v > w {
			return Value{Raw: v}, true
		}
		return Value{Raw: w}, true
	case "Sort":
		return Value{}, true
	case "Minor", "Patch":
		return Value{Raw: strArg(0)}, true
	}
	return Value{}, false
}

// handleModModuleCall intercepts golang.org/x/mod/module calls (issue #187).
func (interp *Interpreter) handleModModuleCall(name string, args []Value) (Value, bool) {
	switch name {
	case "CheckPath", "CheckImportPath", "CheckFilePath", "CheckPathMajor":
		return Value{}, true // nil error
	case "EscapePath":
		escaped := ""
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				escaped = s
			}
		}
		return Value{Raw: []Value{{Raw: escaped}, {}}}, true
	case "EscapeVersion":
		v := ""
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				v = s
			}
		}
		return Value{Raw: []Value{{Raw: v}, {}}}, true
	case "UnescapePath", "UnescapeVersion":
		s := ""
		if len(args) > 0 {
			if sv, ok := args[0].Raw.(string); ok {
				s = sv
			}
		}
		return Value{Raw: []Value{{Raw: s}, {}}}, true
	case "CanonicalVersion":
		s := ""
		if len(args) > 0 {
			if sv, ok := args[0].Raw.(string); ok {
				s = sv
			}
		}
		return Value{Raw: s}, true
	case "IsPseudoVersion", "IsZeroPseudoVersion":
		return Value{Raw: false}, true
	case "MatchPathMajor", "MatchPrefixPatterns":
		return Value{Raw: false}, true
	case "PathMajorPrefix":
		return Value{Raw: ""}, true
	case "PseudoVersion":
		return Value{Raw: ""}, true
	case "PseudoVersionBase", "PseudoVersionRev", "PseudoVersionTime":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "SortVersions":
		return Value{}, true
	}
	return Value{}, false
}

// handleModModfileCall intercepts golang.org/x/mod/modfile calls (issue #187).
func (interp *Interpreter) handleModModfileCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Parse", "ParseLax":
		// (*File, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Format":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "AutoQuote":
		s := ""
		if len(args) > 0 {
			if sv, ok := args[0].Raw.(string); ok {
				s = sv
			}
		}
		return Value{Raw: s}, true
	case "ModulePath":
		return Value{Raw: ""}, true
	case "IsDirectoryPath":
		return Value{Raw: false}, true
	case "MustQuote":
		return Value{Raw: false}, true
	// *File methods
	case "AddGoStmt", "AddModuleStmt", "AddRequire", "AddReplace",
		"AddExclude", "DropRequire", "DropReplace", "DropExclude",
		"SetRequire", "SetRequireSeparateIndirect", "Cleanup",
		"AddRetract", "DropRetract", "AddToolchain", "DropToolchain",
		"SortBlocks":
		return Value{}, true
	case "Module", "Go", "Require", "Replace", "Exclude":
		return opaque, true
	}
	return Value{}, false
}

// handleCryptoTopCall intercepts crypto (top-level) package calls (issue #188).
func (interp *Interpreter) handleCryptoTopCall(name string, args []Value) (Value, bool) {
	switch name {
	case "RegisterHash":
		return Value{}, true
	case "SignMessage":
		// (signature []byte, err error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	// crypto.Hash method New() hash.Hash
	case "New":
		return Value{Raw: struct{}{}}, true
	case "Available":
		return Value{Raw: false}, true
	case "Size":
		return Value{Raw: int64(32)}, true
	case "String":
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleCryptoTestCall intercepts testing/cryptotest calls (issue #188).
func (interp *Interpreter) handleCryptoTestCall(name string, args []Value) (Value, bool) {
	switch name {
	case "SetGlobalRandom":
		// SetGlobalRandom(t *testing.T, seed uint64) — noop
		return Value{}, true
	case "TestAEAD", "TestHash", "TestStream", "TestBlock":
		// TestXxx(t *testing.T, ...) — these probe the implementation but we
		// can noop safely since they're testing-only helpers.
		return Value{}, true
	}
	return Value{}, false
}

// ── v0.63.0: golang.org/x/text, x/term, x/crypto extras ─────────────────────

// handleTextCasesCall intercepts golang.org/x/text/cases calls (issue #193).
func (interp *Interpreter) handleTextCasesCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Lower", "Upper", "Title", "Fold":
		// Returns a Caser — opaque transformer object.
		return opaque, true
	// Caser methods: String(s) string, Bytes(b []byte) []byte
	case "String":
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	case "Bytes":
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				return Value{Raw: bv}, true
			}
		}
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handleTextLanguageCall intercepts golang.org/x/text/language calls (issue #193).
func (interp *Interpreter) handleTextLanguageCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Parse", "Make":
		// (Tag, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "MustParse":
		return opaque, true
	case "ParseAcceptLanguage":
		// ([]Tag, []float32, error)
		return Value{Raw: []Value{{Raw: []Value{opaque}}, {Raw: []Value{{Raw: float64(1.0)}}}, {}}}, true
	case "NewMatcher":
		return opaque, true
	case "Match":
		// (Tag, []language.Confidence)
		return Value{Raw: []Value{opaque, {Raw: []Value{}}}}, true
	// Tag methods
	case "String":
		return Value{Raw: "en"}, true
	case "Base":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Region":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Script":
		return Value{Raw: []Value{opaque, {}}}, true
	case "IsRoot":
		return Value{Raw: false}, true
	case "Parent":
		return opaque, true
	case "Compose":
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, false
}

// handleTextTransformCall intercepts golang.org/x/text/transform calls (issue #193).
func (interp *Interpreter) handleTextTransformCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "String":
		// (string, n int, err error) — passthrough src
		src := ""
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				src = s
			}
		}
		return Value{Raw: []Value{{Raw: src}, {Raw: int64(len(src))}, {}}}, true
	case "Bytes":
		// ([]byte, n int, err error)
		var b []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				b = bv
			}
		}
		return Value{Raw: []Value{{Raw: b}, {Raw: int64(len(b))}, {}}}, true
	case "Append":
		// ([]byte, n int, err error)
		var dst []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				dst = bv
			}
		}
		return Value{Raw: []Value{{Raw: dst}, {Raw: int64(0)}, {}}}, true
	case "NewReader":
		return opaque, true
	case "NewWriter":
		return opaque, true
	case "Chain":
		return opaque, true
	case "RemoveFunc":
		return opaque, true
	// Reader/Writer methods
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Reset":
		return Value{}, true
	case "Close":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleTextNormCall intercepts golang.org/x/text/unicode/norm calls (issue #194).
func (interp *Interpreter) handleTextNormCall(name string, args []Value) (Value, bool) {
	switch name {
	case "String":
		// Form.String(s string) string — passthrough
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	case "Bytes":
		// Form.Bytes(b []byte) []byte — passthrough
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				return Value{Raw: bv}, true
			}
		}
		return Value{Raw: []Value{}}, true
	case "IsNormal", "IsNormalString":
		// conservative: return false (assume not normalized)
		return Value{Raw: false}, true
	case "Append", "AppendString":
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				return Value{Raw: bv}, true
			}
		}
		return Value{Raw: []Value{}}, true
	case "Reader":
		return Value{Raw: struct{}{}}, true
	case "Writer":
		return Value{Raw: struct{}{}}, true
	}
	return Value{}, false
}

// handleTextWidthCall intercepts golang.org/x/text/width calls (issue #194).
func (interp *Interpreter) handleTextWidthCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Fold", "Narrow", "Widen":
		// These are package-level Transformer vars accessed as func-like — opaque
		return opaque, true
	case "Lookup":
		// (Properties, size int)
		return Value{Raw: []Value{opaque, {Raw: int64(1)}}}, true
	case "LookupRune":
		return opaque, true
	case "LookupString":
		// (Properties, size int)
		return Value{Raw: []Value{opaque, {Raw: int64(1)}}}, true
	// Properties methods
	case "Kind":
		return Value{Raw: int64(0)}, true
	case "IsWide", "IsNarrow", "IsAmbiguous", "IsNeutral":
		return Value{Raw: false}, true
	// Transformer methods (Fold/Narrow/Widen are Transformer structs)
	case "Transform":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {Raw: false}, {}}}, true
	case "Reset":
		return Value{}, true
	case "String":
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	case "Bytes":
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				return Value{Raw: bv}, true
			}
		}
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handleTextRunesCall intercepts golang.org/x/text/runes calls (issue #194).
func (interp *Interpreter) handleTextRunesCall(gid int64, site, name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Map", "Remove", "If", "ReplaceIllFormed":
		return opaque, true
	case "In", "NotIn", "Predicate":
		return opaque, true
	// Transformer methods
	case "Transform":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {Raw: false}, {}}}, true
	case "Reset":
		return Value{}, true
	}
	return Value{}, false
}

// handleTermCall intercepts golang.org/x/term calls (issue #195).
func (interp *Interpreter) handleTermCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "IsTerminal":
		return Value{Raw: false}, true
	case "GetSize":
		// (width, height int, err error)
		return Value{Raw: []Value{{Raw: int64(80)}, {Raw: int64(24)}, {}}}, true
	case "ReadPassword":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "MakeRaw":
		// (*State, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Restore":
		return Value{Raw: struct{}{}}, true
	case "GetState":
		return Value{Raw: []Value{opaque, {}}}, true
	// Terminal methods (for *Terminal type)
	case "NewTerminal":
		return opaque, true
	case "ReadLine":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "SetPrompt":
		return Value{}, true
	case "SetSize":
		return Value{Raw: struct{}{}}, true
	case "AutoCompleteCallback":
		return Value{}, true
	}
	return Value{}, false
}

// handleChacha20Call intercepts golang.org/x/crypto/chacha20poly1305 calls (issue #195).
func (interp *Interpreter) handleChacha20Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New", "NewX":
		// (cipher.AEAD, error)
		return Value{Raw: []Value{opaque, {}}}, true
	// AEAD interface methods on opaque cipher
	case "NonceSize", "Overhead":
		return Value{Raw: int64(12)}, true
	case "Seal":
		// (dst []byte)
		var dst []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				dst = bv
			}
		}
		return Value{Raw: dst}, true
	case "Open":
		// ([]byte, error)
		var plain []Value
		if len(args) > 2 {
			if bv, ok := args[2].Raw.([]Value); ok {
				plain = bv
			}
		}
		return Value{Raw: []Value{{Raw: plain}, {}}}, true
	}
	return Value{}, false
}

// handleArgon2Call intercepts golang.org/x/crypto/argon2 calls (issue #195).
func (interp *Interpreter) handleArgon2Call(name string, args []Value) (Value, bool) {
	switch name {
	case "Key", "IDKey":
		// Key(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte
		// Return opaque byte slice (no actual KDF computation in interpreter).
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handleSSHCall intercepts golang.org/x/crypto/ssh calls (issue #196).
func (interp *Interpreter) handleSSHCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Dial":
		// (client *Client, err error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewClientConn":
		// (Conn, <-chan NewChannel, <-chan *Request, error)
		return Value{Raw: []Value{opaque, opaque, opaque, {}}}, true
	case "NewServerConn":
		// (*ServerConn, <-chan NewChannel, <-chan *Request, error)
		return Value{Raw: []Value{opaque, opaque, opaque, {}}}, true
	case "NewSignerFromKey", "NewPublicKey":
		// (interface{}/PublicKey, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParsePrivateKey", "ParseRawPrivateKey":
		// (Signer, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "ParseAuthorizedKey":
		// (out PublicKey, comment string, options []string, rest []byte, err error)
		return Value{Raw: []Value{opaque, {Raw: ""}, {Raw: []Value{}}, {Raw: []Value{}}, {}}}, true
	case "ParseKnownHosts":
		// returns a func — opaque
		return opaque, true
	case "ParsePublicKey":
		// (PublicKey, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "MarshalAuthorizedKey":
		return Value{Raw: []Value{}}, true
	case "FingerprintSHA256", "FingerprintLegacyMD5":
		return Value{Raw: ""}, true
	case "InsecureIgnoreHostKey", "FixedHostKey":
		// HostKeyCallback functions — opaque func
		return opaque, true
	// *Client methods
	case "NewSession":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Close":
		return Value{Raw: struct{}{}}, true
	case "Listen":
		return Value{Raw: []Value{opaque, {}}}, true
	case "OpenChannel":
		return Value{Raw: []Value{opaque, opaque, {}}}, true
	case "SendRequest":
		return Value{Raw: []Value{{Raw: false}, {Raw: []Value{}}, {}}}, true
	// *Session methods
	case "Run", "Start", "Shell":
		return Value{Raw: struct{}{}}, true
	case "Output", "CombinedOutput":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Wait":
		return Value{Raw: struct{}{}}, true
	case "RequestPty":
		return Value{Raw: struct{}{}}, true
	case "StdinPipe":
		return Value{Raw: []Value{opaque, {}}}, true
	case "StdoutPipe", "StderrPipe":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Setenv":
		return Value{Raw: struct{}{}}, true
	// Key exchange / cipher names noop
	case "NewCertSigner":
		return Value{Raw: []Value{opaque, {}}}, true
	}
	return Value{}, false
}

// ── v0.64.0: x/crypto NaCl + Blake2 + ed25519; x/text encoding + collate + search ──

// handleNaclBoxCall intercepts golang.org/x/crypto/nacl/box calls (issue #197).
func (interp *Interpreter) handleNaclBoxCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "GenerateKey":
		// (publicKey *[32]byte, privateKey *[32]byte, err error)
		return Value{Raw: []Value{opaque, opaque, {}}}, true
	case "Precompute":
		// func Precompute(sharedKey, peersPublicKey, privateKey *[32]byte) — noop
		return Value{}, true
	case "Seal", "SealAfterPrecomputation":
		// ([]byte)
		var out []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				out = bv
			}
		}
		return Value{Raw: out}, true
	case "SealAnonymous":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Open", "OpenAfterPrecomputation", "OpenAnonymous":
		// ([]byte, bool)
		return Value{Raw: []Value{{Raw: []Value{}}, {Raw: true}}}, true
	}
	return Value{}, false
}

// handleNaclSecretboxCall intercepts golang.org/x/crypto/nacl/secretbox calls (issue #197).
func (interp *Interpreter) handleNaclSecretboxCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Seal":
		var out []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				out = bv
			}
		}
		return Value{Raw: out}, true
	case "Open":
		// ([]byte, bool)
		return Value{Raw: []Value{{Raw: []Value{}}, {Raw: true}}}, true
	}
	return Value{}, false
}

// handleCurve25519Call intercepts golang.org/x/crypto/curve25519 calls (issue #197).
func (interp *Interpreter) handleCurve25519Call(name string, args []Value) (Value, bool) {
	switch name {
	case "X25519":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "ScalarBaseMult", "ScalarMult":
		// func ScalarBaseMult(dst, scalar *[32]byte) — noop
		return Value{}, true
	}
	return Value{}, false
}

// handlePoly1305Call intercepts golang.org/x/crypto/poly1305 calls (issue #197).
func (interp *Interpreter) handlePoly1305Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		// *MAC
		return opaque, true
	case "Sum":
		// func Sum(out *[16]byte, m []byte, key *[32]byte) — noop (modifies out in place)
		return Value{}, true
	case "Verify":
		// func Verify(mac *[16]byte, m []byte, key *[32]byte) bool
		return Value{Raw: false}, true
	// *MAC methods
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Size":
		return Value{Raw: int64(16)}, true
	case "Sum_":
		// Sum(b []byte) []byte — disambiguated from package-level Sum
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// handleBlake2bCall intercepts golang.org/x/crypto/blake2b calls (issue #198).
func (interp *Interpreter) handleBlake2bCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New", "New256", "New384", "New512":
		// (hash.Hash, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "NewXOF":
		// (XOF, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Sum256":
		// [Size256]byte — return opaque
		return opaque, true
	case "Sum384":
		return opaque, true
	case "Sum512":
		return opaque, true
	// hash.Hash / XOF methods on opaque
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Sum":
		var b []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				b = bv
			}
		}
		return Value{Raw: b}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(64)}, true
	case "BlockSize":
		return Value{Raw: int64(128)}, true
	case "Read":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Clone":
		return opaque, true
	}
	return Value{}, false
}

// handleBlake2sCall intercepts golang.org/x/crypto/blake2s calls (issue #198).
func (interp *Interpreter) handleBlake2sCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New128", "New256":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Sum256":
		return opaque, true
	case "NewXOF":
		return Value{Raw: []Value{opaque, {}}}, true
	// hash.Hash methods
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Sum":
		var b []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				b = bv
			}
		}
		return Value{Raw: b}, true
	case "Reset":
		return Value{}, true
	case "Size":
		return Value{Raw: int64(32)}, true
	case "BlockSize":
		return Value{Raw: int64(64)}, true
	}
	return Value{}, false
}

// handleXEd25519Call intercepts golang.org/x/crypto/ed25519 calls (issue #198).
// This package is a thin alias for crypto/ed25519; intercept all its functions.
func (interp *Interpreter) handleXEd25519Call(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "GenerateKey":
		// (PublicKey, PrivateKey, error)
		return Value{Raw: []Value{opaque, opaque, {}}}, true
	case "NewKeyFromSeed":
		// PrivateKey
		return opaque, true
	case "Sign":
		// []byte
		return Value{Raw: []Value{}}, true
	case "Verify":
		return Value{Raw: false}, true
	// PrivateKey/PublicKey methods
	case "Public":
		return opaque, true
	case "Equal":
		return Value{Raw: false}, true
	case "Seed":
		return Value{Raw: []Value{}}, true
	case "Sign_":
		// PrivateKey.Sign(rand, msg, opts) ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, false
}

// handleTextEncodingCall intercepts golang.org/x/text/encoding calls (issue #199).
func (interp *Interpreter) handleTextEncodingCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "HTMLEscapeUnsupported", "ReplaceUnsupported":
		// (*Encoder) — wraps an existing encoder
		return opaque, true
	// Decoder/Encoder methods
	case "NewDecoder":
		return opaque, true
	case "NewEncoder":
		return opaque, true
	case "Transform":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: int64(0)}, {Raw: false}, {}}}, true
	case "Reset":
		return Value{}, true
	}
	return Value{}, false
}

// handleTextCharmapCall intercepts golang.org/x/text/encoding/charmap calls (issue #199).
func (interp *Interpreter) handleTextCharmapCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewDecoder":
		return opaque, true
	case "NewEncoder":
		return opaque, true
	case "String":
		return Value{Raw: ""}, true
	case "ID":
		// (identifier.MIB, string)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: ""}}}, true
	case "DecodeByte":
		return Value{Raw: int64(0)}, true // rune
	case "EncodeRune":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: false}}}, true // (byte, bool)
	}
	return Value{}, false
}

// handleTextEncodingUnicodeCall intercepts golang.org/x/text/encoding/unicode calls (issue #199).
func (interp *Interpreter) handleTextEncodingUnicodeCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "UTF16":
		// encoding.Encoding
		return opaque, true
	case "BOMOverride":
		// transform.Transformer
		return opaque, true
	// Encoding methods
	case "NewDecoder":
		return opaque, true
	case "NewEncoder":
		return opaque, true
	case "String":
		return Value{Raw: "UTF-16"}, true
	case "ID":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleTextCollateCall intercepts golang.org/x/text/collate calls (issue #200).
func (interp *Interpreter) handleTextCollateCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New", "NewFromTable":
		return opaque, true
	case "Supported":
		// []language.Tag — return empty slice
		return Value{Raw: []Value{}}, true
	case "OptionsFromTag":
		return opaque, true
	case "Reorder":
		return opaque, true
	// *Collator methods
	case "Compare", "CompareString":
		// 0 = equal (conservative)
		return Value{Raw: int64(0)}, true
	case "Key", "KeyFromString":
		// []byte — empty
		return Value{Raw: []Value{}}, true
	case "Sort", "SortStrings":
		// noop
		return Value{}, true
	}
	return Value{}, false
}

// handleTextSearchCall intercepts golang.org/x/text/search calls (issue #200).
func (interp *Interpreter) handleTextSearchCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		return opaque, true
	// *Matcher methods
	case "Index", "IndexString":
		// (start, end int) — -1,-1 means not found (conservative)
		return Value{Raw: []Value{{Raw: int64(-1)}, {Raw: int64(-1)}}}, true
	case "Equal", "EqualString":
		return Value{Raw: false}, true
	case "Compile", "CompileString":
		return opaque, true
	// *Pattern methods
	case "FindAllIndex":
		return Value{Raw: []Value{}}, true
	}
	return Value{}, false
}

// ── v0.65.0: x/crypto stream ciphers + KDF; x/text message/number/currency/bidi/precis ──

// handleScryptCall intercepts golang.org/x/crypto/scrypt calls (issue #201).
func (interp *Interpreter) handleScryptCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Key":
		// Key(password, salt []byte, N, r, p, keyLen int) ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, false
}

// handleChacha20PrimCall intercepts golang.org/x/crypto/chacha20 calls (issue #201).
func (interp *Interpreter) handleChacha20PrimCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewUnauthenticatedCipher":
		// (*Cipher, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "HChaCha20":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	// *Cipher methods
	case "XORKeyStream":
		// noop (modifies dst in place)
		return Value{}, true
	case "SetCounter":
		return Value{}, true
	case "Advance":
		return Value{}, true
	}
	return Value{}, false
}

// handleXTSCall intercepts golang.org/x/crypto/xts calls (issue #201).
func (interp *Interpreter) handleXTSCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewCipher":
		// (*Cipher, error)
		return Value{Raw: []Value{opaque, {}}}, true
	// *Cipher methods
	case "Encrypt", "Decrypt":
		// noop (modifies dst in place)
		return Value{}, true
	}
	return Value{}, false
}

// handleSalsa20Call intercepts golang.org/x/crypto/salsa20 calls (issue #201).
func (interp *Interpreter) handleSalsa20Call(name string, args []Value) (Value, bool) {
	switch name {
	case "XORKeyStream":
		// func XORKeyStream(out, in []byte, nonce []byte, key *[32]byte) — noop
		return Value{}, true
	}
	return Value{}, false
}

// handleTextMessageCall intercepts golang.org/x/text/message calls (issue #202).
func (interp *Interpreter) handleTextMessageCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewPrinter":
		return opaque, true
	case "MatchLanguage":
		return opaque, true
	case "SetString", "Set":
		return Value{Raw: struct{}{}}, true
	// *Printer methods
	case "Print", "Println":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Printf":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Fprint", "Fprintln", "Fprintf":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Sprint", "Sprintln":
		return Value{Raw: ""}, true
	case "Sprintf":
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleTextNumberCall intercepts golang.org/x/text/number calls (issue #202).
func (interp *Interpreter) handleTextNumberCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Decimal", "Scientific", "Engineering", "Percent":
		// FormatFunc(x) — return opaque
		return opaque, true
	case "Scale", "MaxIntegerDigits", "MaxFractionDigits", "MinIntegerDigits",
		"MinFractionDigits", "IncrementString", "Increment":
		return opaque, true
	}
	return Value{}, false
}

// handleTextCurrencyCall intercepts golang.org/x/text/currency calls (issue #202).
func (interp *Interpreter) handleTextCurrencyCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "MustParseISO":
		return opaque, true
	case "ParseISO":
		// (Unit, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "NarrowSymbol", "Symbol", "ISO":
		return opaque, true
	case "String":
		return Value{Raw: ""}, true
	// Unit methods
	case "Amount":
		return opaque, true
	}
	return Value{}, false
}

// handleTextBidiCall intercepts golang.org/x/text/unicode/bidi calls (issue #203).
func (interp *Interpreter) handleTextBidiCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "AppendReverse":
		// func AppendReverse(out, in []byte) []byte — passthrough in
		var in []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				in = bv
			}
		}
		return Value{Raw: in}, true
	case "ReverseString":
		// func ReverseString(s string) string — passthrough
		if len(args) > 0 {
			if s, ok := args[0].Raw.(string); ok {
				return Value{Raw: s}, true
			}
		}
		return Value{Raw: ""}, true
	// *Paragraph methods
	case "SetBytes", "SetString":
		return Value{Raw: []Value{opaque, {}}}, true
	case "Order":
		return Value{Raw: []Value{opaque, {}}}, true
	// *Ordering methods
	case "NumRuns":
		return Value{Raw: int64(1)}, true
	case "Run":
		return opaque, true
	case "Direction":
		return Value{Raw: int64(0)}, true // LeftToRight
	// Package-level property lookups
	case "LookupRune", "Lookup":
		// Returns (Properties, size int) — return opaque properties + 1
		return Value{Raw: []Value{stdlibOpaque, {Raw: int64(1)}}}, true
	case "LookupString":
		// Returns (Properties, size int)
		return Value{Raw: []Value{stdlibOpaque, {Raw: int64(1)}}}, true
	// Properties methods
	case "Class", "IsBracket", "IsOpeningBracket", "HasStrongType", "IsMirror":
		return Value{Raw: int64(0)}, true
	}
	return Value{}, false
}

// handleRuneNamesCall intercepts golang.org/x/text/unicode/runenames calls (issue #203).
func (interp *Interpreter) handleRuneNamesCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Name":
		// func Name(r rune) string — return empty (no UCD data)
		return Value{Raw: ""}, true
	}
	return Value{}, false
}

// handleBidiRuleCall intercepts golang.org/x/text/secure/bidirule calls (issue #203).
func (interp *Interpreter) handleBidiRuleCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Direction":
		// func Direction(b []byte) bidi.Direction — conservative: LeftToRight (0)
		return Value{Raw: int64(0)}, true
	case "DirectionString":
		return Value{Raw: int64(0)}, true
	case "Valid", "ValidString":
		// conservative: assume valid
		return Value{Raw: true}, true
	}
	return Value{}, false
}

// handlePrecisCall intercepts golang.org/x/text/secure/precis calls (issue #204).
func (interp *Interpreter) handlePrecisCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewFreeform", "NewIdentifier", "NewRestrictedProfile":
		return opaque, true
	// *Profile methods
	case "String":
		// *Profile.String(s string) (string, error)
		src := ""
		if len(args) > 1 {
			if s, ok := args[1].Raw.(string); ok {
				src = s
			}
		}
		return Value{Raw: []Value{{Raw: src}, {}}}, true
	case "Bytes":
		// *Profile.Bytes(b []byte) ([]byte, error)
		var b []Value
		if len(args) > 1 {
			if bv, ok := args[1].Raw.([]Value); ok {
				b = bv
			}
		}
		return Value{Raw: []Value{{Raw: b}, {}}}, true
	case "Append", "AppendCompareKey":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Compare":
		return Value{Raw: false}, true
	case "CompareKey":
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "NewTransformer":
		return opaque, true
	case "Allowed":
		return opaque, true
	}
	return Value{}, false
}

// handleTextEncodingJapaneseCall intercepts golang.org/x/text/encoding/japanese calls (issue #204).
func (interp *Interpreter) handleTextEncodingJapaneseCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewDecoder":
		return opaque, true
	case "NewEncoder":
		return opaque, true
	case "String":
		return Value{Raw: ""}, true
	case "ID":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleTextHTMLIndexCall intercepts golang.org/x/text/encoding/htmlindex calls (issue #204).
func (interp *Interpreter) handleTextHTMLIndexCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Get":
		// (encoding.Encoding, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Name":
		// (string, error)
		return Value{Raw: []Value{{Raw: "utf-8"}, {}}}, true
	case "LanguageDefault":
		// string
		return Value{Raw: "utf-8"}, true
	}
	return Value{}, false
}

// handleTextEncodingKoreanCall intercepts golang.org/x/text/encoding/korean calls (issue #205).
func (interp *Interpreter) handleTextEncodingKoreanCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewDecoder", "NewEncoder":
		return opaque, true
	case "String":
		return Value{Raw: ""}, true
	case "ID":
		// (identifier.MIB, string)
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleTextEncodingSimplifiedChineseCall intercepts golang.org/x/text/encoding/simplifiedchinese calls (issue #205).
func (interp *Interpreter) handleTextEncodingSimplifiedChineseCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewDecoder", "NewEncoder":
		return opaque, true
	case "String":
		return Value{Raw: ""}, true
	case "ID":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleTextEncodingTraditionalChineseCall intercepts golang.org/x/text/encoding/traditionalchinese calls (issue #205).
func (interp *Interpreter) handleTextEncodingTraditionalChineseCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "NewDecoder", "NewEncoder":
		return opaque, true
	case "String":
		return Value{Raw: ""}, true
	case "ID":
		return Value{Raw: []Value{{Raw: int64(0)}, {Raw: ""}}}, true
	}
	return Value{}, false
}

// handleTextIANAIndexCall intercepts golang.org/x/text/encoding/ianaindex calls (issue #206).
func (interp *Interpreter) handleTextIANAIndexCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "Encoding":
		// (*Index).Encoding(name string) (encoding.Encoding, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Name":
		// (*Index).Name(e encoding.Encoding) (string, error)
		return Value{Raw: []Value{{Raw: "utf-8"}, {}}}, true
	}
	return Value{}, false
}

// handleNetTraceCall intercepts golang.org/x/net/trace calls (issue #206).
func (interp *Interpreter) handleNetTraceCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "New":
		// func New(family, title string) Trace — opaque interface
		return opaque, true
	case "NewContext":
		// func NewContext(ctx context.Context, tr Trace) context.Context — passthrough ctx
		if len(args) > 0 {
			return args[0], true
		}
		return opaque, true
	case "FromContext":
		// func FromContext(ctx) (Trace, bool)
		return Value{Raw: []Value{opaque, {Raw: false}}}, true
	case "NewEventLog":
		// func NewEventLog(family, title string) EventLog
		return opaque, true
	// Trace interface methods
	case "LazyLog", "LazyPrintf", "SetError", "SetRecycler", "SetTraceInfo", "Finish":
		return Value{}, true
	// EventLog interface methods
	case "Printf", "Errorf":
		return Value{}, true
	// HTTP handlers
	case "Events", "Render", "RenderEvents", "Traces":
		return Value{}, true
	}
	return Value{}, false
}

// handleDNSMessageCall intercepts golang.org/x/net/dns/dnsmessage calls (issue #207).
func (interp *Interpreter) handleDNSMessageCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	// Constructors
	case "MustNewName":
		return opaque, true
	case "NewName":
		// (Name, error)
		return Value{Raw: []Value{opaque, {}}}, true
	// *Message methods
	case "Pack", "AppendPack":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "Unpack":
		return Value{}, true
	case "GoString":
		return Value{Raw: ""}, true
	// *Parser methods
	case "Start":
		// (Header, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "Question", "Answer", "Authority", "Additional":
		// (Resource/Question, error)
		return Value{Raw: []Value{opaque, {}}}, true
	case "AllQuestions", "AllAnswers", "AllAuthorities", "AllAdditionals":
		// ([]T, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	case "SkipQuestion", "SkipAnswer", "SkipAuthority", "SkipAdditional",
		"SkipAllQuestions", "SkipAllAnswers", "SkipAllAuthorities", "SkipAllAdditionals",
		"PopQuestion", "PopAnswer", "PopAuthority", "PopAdditional":
		return Value{}, true
	// *Builder methods
	case "StartQuestions", "StartAnswers", "StartAuthorities", "StartAdditionals":
		return Value{}, true
	case "Finish":
		// ([]byte, error)
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	// Value type methods
	case "String":
		return Value{Raw: ""}, true
	case "Pack_":
		return Value{Raw: []Value{{Raw: []Value{}}, {}}}, true
	}
	return Value{}, false
}

// handleHPACKCall intercepts golang.org/x/net/http2/hpack calls (issue #208).
func (interp *Interpreter) handleHPACKCall(name string, args []Value) (Value, bool) {
	opaque := stdlibOpaque
	switch name {
	case "AppendHuffmanString":
		// func AppendHuffmanString(dst []byte, s string) []byte — passthrough dst
		if len(args) > 0 {
			if bv, ok := args[0].Raw.([]Value); ok {
				return Value{Raw: bv}, true
			}
		}
		return Value{Raw: []Value{}}, true
	case "HuffmanDecode":
		// func HuffmanDecode(w io.Writer, v []byte) (int, error)
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "HuffmanDecodeToString":
		// func HuffmanDecodeToString(v []byte) (string, error)
		return Value{Raw: []Value{{Raw: ""}, {}}}, true
	case "HuffmanEncodeLength":
		// func HuffmanEncodeLength(s string) uint64
		return Value{Raw: uint64(0)}, true
	case "NewDecoder":
		// func NewDecoder(maxDynamicTableSize uint32, emit func(HeaderField)) *Decoder
		return opaque, true
	case "NewEncoder":
		// func NewEncoder(w io.Writer) *Encoder
		return opaque, true
	// *Decoder methods
	case "Write":
		return Value{Raw: []Value{{Raw: int64(0)}, {}}}, true
	case "Close":
		return Value{}, true
	case "SetMaxDynamicTableSize", "SetMaxStringLength", "SetEmitEnabled", "SetEmitFunc":
		return Value{}, true
	// *Encoder methods
	case "WriteField":
		return Value{}, true
	case "SetMaxDynamicTableSizeLimit":
		return Value{}, true
	}
	return Value{}, false
}

// handleSyncMapCall intercepts golang.org/x/sync/syncmap calls (issue #208).
// syncmap.Map is a type alias for sync.Map; intercept its methods here.
func (interp *Interpreter) handleSyncMapCall(name string, args []Value) (Value, bool) {
	switch name {
	case "Store", "Delete", "Swap":
		return Value{}, true
	case "Load", "LoadAndDelete":
		// (value any, loaded bool)
		return Value{Raw: []Value{{}, {Raw: false}}}, true
	case "LoadOrStore":
		// (actual any, loaded bool) — return second arg as actual
		var actual Value
		if len(args) > 1 {
			actual = args[1]
		}
		return Value{Raw: []Value{actual, {Raw: false}}}, true
	case "Range":
		// Range(f func(key, value any) bool) — noop (no iteration model)
		return Value{}, true
	case "CompareAndSwap":
		return Value{Raw: false}, true
	case "CompareAndDelete":
		return Value{Raw: false}, true
	case "Clear":
		return Value{}, true
	}
	return Value{}, false
}
