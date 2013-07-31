// Copyright 2012 chaishushan@gmail.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base58

import (
        "crypto/sha256"
)

func Hash(ba []byte) []byte {
        sha := sha256.New()
        sha2 := sha256.New() // hash twice
        ba = sha.Sum(ba)
        return sha2.Sum(ba)
}