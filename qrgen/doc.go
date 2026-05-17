// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package qrgen generates QR codes from arbitrary text input.
//
// The encoder is implemented from scratch following ISO/IEC 18004:2015,
// with no runtime dependencies beyond the Go standard library. See the
// project README and docs/theory/ for the algorithms used.
//
// This package is in early development; the public API is unstable until
// v0.1.0 is tagged.
package qrgen
