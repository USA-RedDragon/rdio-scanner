// Copyright (C) 2019-2022 Chrystian Huot <chrystian.huot@saubeo.solutions>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>

//go:build tools

// Package tools pins the ent code generator as a module dependency so that
// `go generate ./ent` resolves it from go.mod/go.sum without the -mod=mod
// flag (which would rewrite go.sum and fail goreleaser's clean-tree check).
package main

import _ "entgo.io/ent/cmd/ent"
