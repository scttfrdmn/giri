// text_encoding_cjk exercises x/text/encoding/korean, simplifiedchinese,
// and traditionalchinese intercepts (issue #205). Expected: 0 violations.
package main

import (
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

func main() {
	// korean: EUCKR
	dec := korean.EUCKR.NewDecoder()
	_ = dec
	enc := korean.EUCKR.NewEncoder()
	_ = enc

	// simplifiedchinese: GB18030
	dec2 := simplifiedchinese.GB18030.NewDecoder()
	_ = dec2
	enc2 := simplifiedchinese.GB18030.NewEncoder()
	_ = enc2

	// simplifiedchinese: GBK
	dec3 := simplifiedchinese.GBK.NewDecoder()
	_ = dec3

	// simplifiedchinese: HZGB2312
	dec4 := simplifiedchinese.HZGB2312.NewDecoder()
	_ = dec4

	// traditionalchinese: Big5
	dec5 := traditionalchinese.Big5.NewDecoder()
	_ = dec5
	enc5 := traditionalchinese.Big5.NewEncoder()
	_ = enc5
}
