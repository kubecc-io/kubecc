/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package types

const (
	Clang         = ToolchainKind_ToolchainKind_Clang
	Gnu           = ToolchainKind_ToolchainKind_Gnu
	TestToolchain = ToolchainKind_ToolchainKind_Test
	Sleep         = ToolchainKind_ToolchainKind_Sleep

	C     = ToolchainLang_ToolchainLang_C
	CXX   = ToolchainLang_ToolchainLang_CXX
	Multi = ToolchainLang_ToolchainLang_Multi

	Agent         = Component_Component_Agent
	Scheduler     = Component_Component_Scheduler
	Controller    = Component_Component_Controller
	Consumer      = Component_Component_Consumer
	Consumerd     = Component_Component_Consumerd
	Make          = Component_Component_Make
	CLI           = Component_Component_CLI
	Dashboard     = Component_Component_Dashboard
	Monitor       = Component_Component_Monitor
	Cache         = Component_Component_Cache
	TestComponent = Component_Component_Test

	Unknown = StorageLocation_StorageLocation_Unknown
	Memory  = StorageLocation_StorageLocation_Memory
	Disk    = StorageLocation_StorageLocation_Disk
	S3      = StorageLocation_StorageLocation_S3
)
