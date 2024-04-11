package data

import _ "embed"

//go:embed header.bin
var Header []byte

//go:embed footer.bin
var Footer []byte
