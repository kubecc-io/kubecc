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
