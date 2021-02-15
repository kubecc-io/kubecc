package types

var (
	Clang = ToolchainKind_Clang_
	Gnu   = ToolchainKind_Gnu_

	C     = ToolchainLang_C_
	CXX   = ToolchainLang_CXX_
	Multi = ToolchainLang_Multi_

	Agent      = Component_Agent_
	Scheduler  = Component_Scheduler_
	Controller = Component_Controller_
	Consumer   = Component_Consumer_
	Consumerd  = Component_Consumerd_
	Make       = Component_Make_
	Test       = Component_Test_
	Dashboard  = Component_Dashboard_
)
