//go:build !go1.19 && go1.18

package gcexportdata

import (
	_ "embed"
)

//go:embed go118_darwin_amd64.zip
var exportFilesZIP []byte
