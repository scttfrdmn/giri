package interpreter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
)

var integrationTests = []struct {
	name           string
	dir            string
	wantViolations int
	wantCategory   string // empty = don't check; matches error message substring OR report.CategoryFor(v)
	config         interpreter.Config
}{
	{
		name:           "safe alloc",
		dir:            "safe_alloc",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe oob",
		dir:            "unsafe_oob",
		wantViolations: 1,
		wantCategory:   "unsafe",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "binop",
		dir:            "binop",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "multi return",
		dir:            "multi_return",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "uintptr gc",
		dir:            "uintptr_gc",
		wantViolations: 1,
		wantCategory:   "rule 2",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "safe uintptr",
		dir:            "safe_uintptr",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "misaligned ptr",
		dir:            "misaligned_ptr",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	// v0.3.1 regression tests
	{
		name:           "loop phi zero",
		dir:            "loop",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "closure freevars",
		dir:            "closure",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "maxsteps enforced",
		dir:            "maxsteps",
		wantViolations: 1,
		wantCategory:   "execution limit",
		config: func() interpreter.Config {
			c := interpreter.DefaultConfig()
			c.MaxSteps = 200 // trip well before 1M iterations
			return c
		}(),
	},
	{
		name:           "panic defers",
		dir:            "panic_defers",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.4.0 regression tests
	{
		name:           "data race",
		dir:            "data_race",
		wantViolations: 1,
		wantCategory:   "data race",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "no race chan",
		dir:            "no_race_chan",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "uninit read",
		dir:            "uninit_read",
		wantViolations: 1,
		wantCategory:   "uninitialized",
		config: func() interpreter.Config {
			c := interpreter.DefaultConfig()
			c.TrackInit = true
			return c
		}(),
	},
	// v0.5.0 regression tests
	{
		name:           "spawn hb",
		dir:            "spawn_hb",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "nil deref",
		dir:            "nil_deref",
		wantViolations: 1,
		wantCategory:   "nil pointer",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "close panic",
		dir:            "close_panic",
		wantViolations: 1,
		wantCategory:   "closed channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "slice oob",
		dir:            "slice_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	// v0.7.0 regression tests
	{
		name:           "type assert ok",
		dir:            "type_assert_ok",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "type assert fail",
		dir:            "type_assert_fail",
		wantViolations: 1,
		wantCategory:   "type-assertion",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "iface dispatch",
		dir:            "iface_dispatch",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.8.0 regression tests
	{
		name:           "reflect uintptr gc",
		dir:            "reflect_uintptr",
		wantViolations: 1,
		wantCategory:   "rule 5",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "slice header",
		dir:            "slice_header",
		wantViolations: 1,
		wantCategory:   "rule 6",
		config:         interpreter.DefaultConfig(),
	},
	// v0.9.0 regression tests
	{
		name:           "callstack depth",
		dir:            "callstack_depth",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "goroutine leak",
		dir:            "goroutine_leak",
		wantViolations: 1,
		wantCategory:   "goroutine leak",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "no goroutine leak",
		dir:            "no_goroutine_leak",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.10.0 regression tests
	{
		name:           "type switch dispatch",
		dir:            "type_switch_dispatch",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "type switch nil",
		dir:            "type_switch_nil",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.11.0 regression tests
	{
		name:           "strings intercept",
		dir:            "strings_intercept",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "strconv atoi",
		dir:            "strconv_atoi",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "fmt sprintf",
		dir:            "fmt_sprintf",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	// v0.12.0 regression tests
	{
		name:           "buffered chan overflow",
		dir:            "buffered_chan_overflow",
		wantViolations: 1,
		wantCategory:   "goroutine leak",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "select default",
		dir:            "select_default",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "select timeout",
		dir:            "select_timeout",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "map race",
		dir:            "map_race",
		wantViolations: 1,
		wantCategory:   "data race",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sync map no race",
		dir:            "sync_map_no_race",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.46.0 regression tests
	{
		name:           "complex neg",
		dir:            "complex_neg",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "complex conv",
		dir:            "complex_conv",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "select recv ok",
		dir:            "select_recv_ok",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "select recv closed",
		dir:            "select_recv_closed",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.45.0 regression tests
	{
		name:           "string to rune",
		dir:            "string_to_rune",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "rune to string",
		dir:            "rune_to_string",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "range chan",
		dir:            "range_chan",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "range chan valid",
		dir:            "range_chan_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.44.0 regression tests
	{
		name:           "and not",
		dir:            "and_not",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "and not valid",
		dir:            "and_not_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "complex builtins",
		dir:            "complex_builtins",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "complex arith",
		dir:            "complex_arith",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.43.0 regression tests
	{
		name:           "len map chan",
		dir:            "len_map_chan",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "len map chan zero",
		dir:            "len_map_chan_zero",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "int truncate",
		dir:            "int_truncate",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "int truncate valid",
		dir:            "int_truncate_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.42.0 regression tests
	{
		name:           "make map neg",
		dir:            "make_map_neg",
		wantViolations: 1,
		wantCategory:   "make-invalid",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "make map valid",
		dir:            "make_map_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "range array",
		dir:            "range_array",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "range array race",
		dir:            "range_array_race",
		wantViolations: 1,
		wantCategory:   "data race",
		config:         interpreter.DefaultConfig(),
	},
	// v0.41.0 regression tests
	{
		name:           "slice elem oob",
		dir:            "slice_elem_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "slice elem valid",
		dir:            "slice_elem_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "make len gt cap",
		dir:            "make_len_gt_cap",
		wantViolations: 1,
		wantCategory:   "make-invalid",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "make len eq cap",
		dir:            "make_len_eq_cap",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.40.0 regression tests
	{
		name:           "array index oob",
		dir:            "array_index_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds", // uses report category style (exercises #132 fix)
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "array index valid",
		dir:            "array_index_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.39.0 regression tests
	{
		name:           "fieldaddr nil struct",
		dir:            "fieldaddr_nil_struct",
		wantViolations: 1,
		wantCategory:   "nil pointer",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "fieldaddr valid",
		dir:            "fieldaddr_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe string neg",
		dir:            "unsafe_string_neg",
		wantViolations: 1,
		wantCategory:   "unsafe-slice",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe string nil",
		dir:            "unsafe_string_nil",
		wantViolations: 1,
		wantCategory:   "unsafe-slice",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe string valid",
		dir:            "unsafe_string_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.38.0 regression tests
	{
		name:           "unsafe slice neg",
		dir:            "unsafe_slice_neg",
		wantViolations: 1,
		wantCategory:   "unsafe-slice",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe slice nil",
		dir:            "unsafe_slice_nil",
		wantViolations: 1,
		wantCategory:   "unsafe-slice",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "unsafe slice valid",
		dir:            "unsafe_slice_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.37.0 regression tests
	{
		name:           "nil slice index",
		dir:            "nil_slice_index",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "slice index valid",
		dir:            "slice_index_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "mutex unlock unowned",
		dir:            "mutex_unlock_unowned",
		wantViolations: 1,
		wantCategory:   "mutex-unlock",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "mutex unlock valid",
		dir:            "mutex_unlock_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.36.0 regression tests
	{
		name:           "string index oob",
		dir:            "string_index_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "string index valid",
		dir:            "string_index_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "negative shift",
		dir:            "negative_shift",
		wantViolations: 1,
		wantCategory:   "negative-shift",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "valid shift",
		dir:            "valid_shift",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.35.0 regression tests
	{
		name:           "nil channel close",
		dir:            "nil_channel_close",
		wantViolations: 1,
		wantCategory:   "nil-channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "nil channel send",
		dir:            "nil_channel_send",
		wantViolations: 1,
		wantCategory:   "nil-channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "nil channel recv",
		dir:            "nil_channel_recv",
		wantViolations: 1,
		wantCategory:   "nil-channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "make invalid len",
		dir:            "make_invalid_len",
		wantViolations: 1,
		wantCategory:   "make-invalid",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "make valid",
		dir:            "make_valid",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.34.0 regression tests
	{
		name:           "context cancel ok",
		dir:            "context_cancel_ok",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "context cancel leak",
		dir:            "context_cancel_leak",
		wantViolations: 1,
		wantCategory:   "context-cancel-leak",
		config:         interpreter.DefaultConfig(),
	},
	// v0.31.0 regression tests
	{
		name: "custom intercept",
		dir:  "custom_intercept",
		// locallib.Compute and locallib.MustAlloc are intercepted via
		// Config.Intercepts; the interpreter never executes their bodies.
		wantViolations: 0,
		wantCategory:   "",
		config: func() interpreter.Config {
			cfg := interpreter.DefaultConfig()
			const localpkg = "github.com/scttfrdmn/giri/pkg/interpreter/testdata/integration/custom_intercept/locallib"
			cfg.Intercepts = interpreter.CustomIntercepts{
				localpkg: {
					"Compute": func(args []interpreter.Value) (interpreter.Value, bool) {
						return interpreter.Value{Raw: int64(0)}, true
					},
					"MustAlloc": func(args []interpreter.Value) (interpreter.Value, bool) {
						return interpreter.Value{Raw: []byte{}}, true
					},
				},
			}
			return cfg
		}(),
	},
	// v0.30.0 regression tests
	{
		name:           "fs embed",
		dir:            "fs_embed",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "zip archive",
		dir:            "zip_archive",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "mime multipart",
		dir:            "mime_multipart",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "aes cipher",
		dir:            "aes_cipher",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.28.0 regression tests
	{
		name:           "tls dial",
		dir:            "tls_dial",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sql query",
		dir:            "sql_query",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "strings reader",
		dir:            "strings_reader",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "testing helper",
		dir:            "testing_helper",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.27.0 regression tests
	{
		name:           "binary readwrite",
		dir:            "binary_readwrite",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "hash crc32",
		dir:            "hash_crc32",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "container list",
		dir:            "container_list",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "math big",
		dir:            "math_big",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.26.0 regression tests
	{
		name:           "time ticker",
		dir:            "time_ticker",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "os file rw",
		dir:            "os_file_rw",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "http client",
		dir:            "http_client",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "signal notify",
		dir:            "signal_notify",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.25.0 regression tests
	{
		name:           "url parse",
		dir:            "url_parse",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "url values",
		dir:            "url_values",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "exec command",
		dir:            "exec_command",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "exec lookpath",
		dir:            "exec_lookpath",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "gzip readwrite",
		dir:            "gzip_readwrite",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "zlib compress",
		dir:            "zlib_compress",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sync pool",
		dir:            "sync_pool",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sync cond",
		dir:            "sync_cond",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.24.0 regression tests
	{
		name:           "slice 3index",
		dir:            "slice_3index",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "reflect type of",
		dir:            "reflect_type_of",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "reflect deep equal",
		dir:            "reflect_deep_equal",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "xml marshal",
		dir:            "xml_marshal",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "csv readall",
		dir:            "csv_readall",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "flag parse",
		dir:            "flag_parse",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "runtime numcpu",
		dir:            "runtime_numcpu",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.23.0 regression tests
	{
		name:           "hex encode",
		dir:            "hex_encode",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "base64 encode",
		dir:            "base64_encode",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "crypto rand read",
		dir:            "crypto_rand_read",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "hash sha256",
		dir:            "hash_sha256",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "filepath join",
		dir:            "filepath_join",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "path basic",
		dir:            "path_basic",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "net parse",
		dir:            "net_parse",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "template execute",
		dir:            "template_execute",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.22.0 regression tests
	{
		name:           "atomic counter",
		dir:            "atomic_counter",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "atomic cas",
		dir:            "atomic_cas",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "io readall",
		dir:            "io_readall",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "bufio scanner",
		dir:            "bufio_scanner",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "strings builder",
		dir:            "strings_builder",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "bytes buffer",
		dir:            "bytes_buffer",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "log print",
		dir:            "log_print",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "log fatal",
		dir:            "log_fatal",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.21.0 regression tests
	{
		name:           "string byte index",
		dir:            "string_byte_index",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "string range utf8",
		dir:            "string_range_utf8",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "convert string rune",
		dir:            "convert_string_rune",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "convert bytes string",
		dir:            "convert_bytes_string",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "utf8 rune count",
		dir:            "utf8_rune_count",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "context basic",
		dir:            "context_basic",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.20.0 regression tests
	{
		name:           "min max builtins",
		dir:            "min_max_builtins",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "clear map",
		dir:            "clear_map",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "json marshal",
		dir:            "json_marshal",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "regexp match",
		dir:            "regexp_match",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "math ops",
		dir:            "math_ops",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.19.0 regression tests
	{
		name:           "fmt print return",
		dir:            "fmt_print_return",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "bytes ops",
		dir:            "bytes_ops",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "errors new",
		dir:            "errors_new",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sort slice",
		dir:            "sort_slice",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "sort strings",
		dir:            "sort_strings",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.18.0 regression tests
	{
		name:           "sync once",
		dir:            "sync_once",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "os exit",
		dir:            "os_exit",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "os getenv",
		dir:            "os_getenv",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "delete race",
		dir:            "delete_race",
		wantViolations: 1,
		wantCategory:   "data race",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "safe delete",
		dir:            "safe_delete",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "rand intn",
		dir:            "rand_intn",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.16.0 regression tests
	{
		name:           "safe stack alloc",
		dir:            "safe_stack_alloc",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "bounded value store",
		dir:            "bounded_value_store",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	// v0.17.0 regression tests
	{
		name:           "suppress ignore",
		dir:            "suppress_ignore",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "multi pkg",
		dir:            "multi_pkg",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	// v0.15.0 regression tests
	{
		name:           "deadlock",
		dir:            "deadlock",
		wantViolations: 1,
		wantCategory:   "deadlock",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "wg negative",
		dir:            "wg_negative",
		wantViolations: 1,
		wantCategory:   "waitgroup",
		config:         interpreter.DefaultConfig(),
	},
	// v0.14.0 regression tests
	{
		name:           "double close",
		dir:            "double_close",
		wantViolations: 1,
		wantCategory:   "closed channel",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "nil map write",
		dir:            "nil_map_write",
		wantViolations: 1,
		wantCategory:   "nil map",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "div zero",
		dir:            "div_zero",
		wantViolations: 1,
		wantCategory:   "division by zero",
		config:         interpreter.DefaultConfig(),
	},
	// v0.13.0 regression tests
	{
		name:           "defer unlock",
		dir:            "defer_unlock",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "defer user func",
		dir:            "defer_user_func",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "multi recover",
		dir:            "multi_recover",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
	{
		name:           "named return defer",
		dir:            "named_return_defer",
		wantViolations: 0,
		wantCategory:   "",
		config:         interpreter.DefaultConfig(),
	},
}

var showcaseTests = []struct {
	name           string
	dir            string
	wantViolations int
	wantCategory   string
	config         interpreter.Config
	runs           int   // >0 → use RunN instead of Run
	seed           int64 // seed for RunN
}{
	{
		// unsafe.Add moves pointer past end of [4]byte allocation.
		// go vet: pass, go test -race: pass.
		name:           "unsafe oob",
		dir:            "unsafe_oob",
		wantViolations: 1,
		wantCategory:   "out-of-bounds",
		config:         interpreter.DefaultConfig(),
	},
	{
		// Converts *byte at offset 1 to *uint32 (requires 4-byte alignment).
		// go vet: pass, go test -race: pass.
		name:           "unsafe alignment",
		dir:            "unsafe_alignment",
		wantViolations: 1,
		wantCategory:   "rule 1",
		config:         interpreter.DefaultConfig(),
	},
	{
		// uintptr held across doWork() GC safepoint.
		// go vet: pass, go test -race: pass.
		name:           "uintptr gc hazard",
		dir:            "uintptr_gc_hazard",
		wantViolations: 1,
		wantCategory:   "rule 2",
		config:         interpreter.DefaultConfig(),
	},
	{
		// Reads new(AuthToken).value[0] before any write (TrackInit mode).
		// go vet: pass, go test -race: pass.
		name: "uninit read",
		dir:  "uninit_read",
		config: func() interpreter.Config {
			c := interpreter.DefaultConfig()
			c.TrackInit = true
			return c
		}(),
		wantViolations: 1,
		wantCategory:   "uninitialized",
	},
	{
		// getPort("ftp") dereferences nil from map miss.
		// go vet: pass, go test -race: pass if "ftp" path not covered.
		name:           "nil deref",
		dir:            "nil_deref",
		wantViolations: 1,
		wantCategory:   "nil pointer",
		config:         interpreter.DefaultConfig(),
	},
	{
		// makeAnimal("cat") returns *Cat; a.(*Dog) panics at runtime.
		// go vet: pass (can't statically trace makeAnimal's return type).
		// go test -race: pass (single goroutine, no concurrent access).
		name:           "type assert panic",
		dir:            "type_assert",
		wantViolations: 1,
		wantCategory:   "type-assertion",
		config:         interpreter.DefaultConfig(),
	},
	{
		// processValue() calls v.Pointer() then doWork() before converting back.
		// go vet: pass (types are correct).
		// go test -race: pass (no concurrent access).
		// Giri: reflect.Value.Pointer() uintptr escapes past a GC safepoint (Rule 5).
		name:           "reflect unsafe",
		dir:            "reflect_unsafe",
		wantViolations: 1,
		wantCategory:   "rule 5",
		config:         interpreter.DefaultConfig(),
	},
	{
		// worker() reads from results channel that main never sends on.
		// go vet: pass (channel operations are type-correct).
		// go test -race: pass (no concurrent data access).
		// Giri: goroutine is permanently blocked — goroutine leak.
		name:           "goroutine leak",
		dir:            "goroutine_leak",
		wantViolations: 1,
		wantCategory:   "goroutine leak",
		config:         interpreter.DefaultConfig(),
	},
	{
		// Both goroutines block on the same channel with no sender.
		// go vet: pass, go test -race: pass.
		// Giri: all goroutines are asleep — global deadlock.
		name:           "deadlock",
		dir:            "deadlock",
		wantViolations: 1,
		wantCategory:   "deadlock",
		config:         interpreter.DefaultConfig(),
	},
	{
		// process() workers call Done() but the caller also calls Done(),
		// driving the WaitGroup counter negative when goroutines finish.
		// go vet: pass, go test -race: pass.
		// Giri: intercepts Done() and detects counter < 0.
		name:           "wg negative",
		dir:            "wg_negative",
		wantViolations: 1,
		wantCategory:   "waitgroup",
		config:         interpreter.DefaultConfig(),
	},
	{
		// work() calls Done() without a prior Add(). Round-robin always runs
		// setup() (higher GID) first: Add(1) then Done → counter=0, no fault.
		// PCT sometimes runs work() (lower GID) first: Done → counter=-1 →
		// WaitGroup negative counter violation.
		// go vet: pass, go test -race: pass.
		// Giri + RunN: waitgroup negative counter found within PCT runs.
		name:           "pct race",
		dir:            "pct_race",
		wantViolations: 1,
		wantCategory:   "waitgroup",
		config:         interpreter.DefaultConfig(),
		runs:           20,
		seed:           42,
	},
}

// TestShowcase validates that each showcase program produces the expected
// violation. These programs compile and pass go vet and go test -race, but
// Giri detects a bug via static SSA interpretation.
func TestShowcase(t *testing.T) {
	for _, tt := range showcaseTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			wd, err := os.Getwd()
			if err != nil {
				t.Skipf("could not get working directory: %v", err)
			}
			// Showcase programs live at project_root/testdata/showcase/
			absPath := filepath.Join(wd, "..", "..", "testdata", "showcase", tt.dir)

			prog, err := ssautil.LoadProgram(absPath)
			if err != nil {
				t.Skipf("skipping %s: could not load program: %v", tt.name, err)
				return
			}
			if prog.Main == nil {
				t.Skipf("skipping %s: no main package found", tt.name)
				return
			}

			var result *interpreter.RunResult
			if tt.runs > 0 {
				result = interpreter.RunN(prog, tt.config, tt.runs, tt.seed)
			} else {
				result = interpreter.Run(prog, tt.config)
			}
			gotViolations := len(result.Violations)

			if tt.wantViolations == 0 {
				if gotViolations != 0 {
					t.Errorf("want 0 violations, got %d:", gotViolations)
					for _, v := range result.Violations {
						t.Logf("  - %v", v)
					}
				}
			} else {
				if gotViolations < tt.wantViolations {
					t.Errorf("want >= %d violations, got %d", tt.wantViolations, gotViolations)
					t.Logf("  violations: %v", result.Violations)
				}
			}

			if tt.wantCategory != "" {
				found := false
				for _, v := range result.Violations {
					if strings.Contains(v.Error(), tt.wantCategory) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("want violation containing %q, got: %v", tt.wantCategory, result.Violations)
				}
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	for _, tt := range integrationTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			wd, err := os.Getwd()
			if err != nil {
				t.Skipf("could not get working directory: %v", err)
			}
			absPath := filepath.Join(wd, "testdata", "integration", tt.dir)

			prog, err := ssautil.LoadProgram(absPath)
			if err != nil {
				t.Skipf("skipping %s: could not load program: %v", tt.name, err)
				return
			}
			if prog.Main == nil {
				t.Skipf("skipping %s: no main package found", tt.name)
				return
			}

			result := interpreter.Run(prog, tt.config)

			// Deduplicate violations for count check
			gotViolations := len(result.Violations)

			if tt.wantViolations == 0 {
				if gotViolations != 0 {
					t.Errorf("want 0 violations, got %d:", gotViolations)
					for _, v := range result.Violations {
						t.Logf("  - %v", v)
					}
				}
			} else {
				if gotViolations < tt.wantViolations {
					t.Errorf("want >= %d violations, got %d", tt.wantViolations, gotViolations)
				}
			}

			if tt.wantCategory != "" {
				found := false
				for _, v := range result.Violations {
					// Accept either an error message substring match (legacy style,
					// e.g. "nil pointer") or an exact report category match (preferred
					// style, e.g. "nil-pointer-deref"). (#132)
					if strings.Contains(v.Error(), tt.wantCategory) ||
						report.CategoryFor(v) == tt.wantCategory {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("want violation with category/message %q, violations: %v", tt.wantCategory, result.Violations)
				}
			}
		})
	}
}

// TestTestMode verifies the giri -test workflow: LoadTestPrograms discovers
// TestXxx functions in _test.go files and RunTests runs each through the
// interpreter independently.
//
// test_mode/lib.go exports SafeAdd (no side effects) and Counter (shared global).
// test_mode/lib_test.go has:
//   - TestSafeAdd: calls SafeAdd(2,3); expects 0 violations.
//   - TestCounterRace: two goroutines race on Counter; expects ≥1 data-race violation.
func TestTestMode(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("could not get working directory: %v", err)
	}
	dir := filepath.Join(wd, "testdata", "integration", "test_mode")

	progs, err := ssautil.LoadTestPrograms([]string{dir})
	if err != nil {
		t.Skipf("LoadTestPrograms: %v", err)
	}
	if len(progs) == 0 {
		t.Fatal("expected at least one test program")
	}

	// Run all test functions. Use the first program (there is only one package).
	results := interpreter.RunTests(progs[0], interpreter.DefaultConfig())

	// Build a name → result index.
	byName := make(map[string]*interpreter.TestRunResult)
	for _, r := range results {
		byName[r.Name] = r
	}

	// TestSafeAdd: no violations.
	safe, ok := byName["TestSafeAdd"]
	if !ok {
		t.Error("TestSafeAdd not found in RunTests results")
	} else if !safe.Passed() {
		t.Errorf("TestSafeAdd: expected 0 violations, got %d: %v", len(safe.Violations), safe.Violations)
	}

	// TestCounterRace: at least one data-race violation.
	race, ok := byName["TestCounterRace"]
	if !ok {
		t.Error("TestCounterRace not found in RunTests results")
	} else if race.Passed() {
		t.Error("TestCounterRace: expected ≥1 violation, got 0")
	} else {
		found := false
		for _, v := range race.Violations {
			if strings.Contains(v.Error(), "data race") || strings.Contains(v.Error(), "race") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("TestCounterRace: expected data-race violation, got: %v", race.Violations)
		}
	}
}
