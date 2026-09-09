// Package arquivos is where this example keeps what people upload.
package arquivos

import "github.com/emersonjoe/trilha/blob"

// Arquivos is the store. Memory here because the example has to run from a
// clone with nothing configured and leave nothing behind; a real application
// writes blob.FromEnv(), which is a directory on a laptop and a bucket in
// production, decided by one environment variable.
var Arquivos = blob.New(blob.NewMemory())
