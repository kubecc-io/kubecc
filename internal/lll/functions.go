package lll

var (
	Desugar = globalLog.Desugar
	Named   = globalLog.Named
	With    = globalLog.With
	Debug   = globalLog.Debug
	Info    = globalLog.Info
	Warn    = globalLog.Warn
	Error   = globalLog.Error
	DPanic  = globalLog.DPanic
	Panic   = globalLog.Panic
	Fatal   = globalLog.Fatal
	Debugf  = globalLog.Debugf
	Infof   = globalLog.Infof
	Warnf   = globalLog.Warnf
	Errorf  = globalLog.Errorf
	DPanicf = globalLog.DPanicf
	Panicf  = globalLog.Panicf
	Fatalf  = globalLog.Fatalf
	Debugw  = globalLog.Debugw
	Infow   = globalLog.Infow
	Warnw   = globalLog.Warnw
	Errorw  = globalLog.Errorw
	DPanicw = globalLog.DPanicw
	Panicw  = globalLog.Panicw
	Fatalw  = globalLog.Fatalw
	Sync    = globalLog.Sync
)

func loadFunctions() {
	Desugar = globalLog.Desugar
	Named = globalLog.Named
	With = globalLog.With
	Debug = globalLog.Debug
	Info = globalLog.Info
	Warn = globalLog.Warn
	Error = globalLog.Error
	DPanic = globalLog.DPanic
	Panic = globalLog.Panic
	Fatal = globalLog.Fatal
	Debugf = globalLog.Debugf
	Infof = globalLog.Infof
	Warnf = globalLog.Warnf
	Errorf = globalLog.Errorf
	DPanicf = globalLog.DPanicf
	Panicf = globalLog.Panicf
	Fatalf = globalLog.Fatalf
	Debugw = globalLog.Debugw
	Infow = globalLog.Infow
	Warnw = globalLog.Warnw
	Errorw = globalLog.Errorw
	DPanicw = globalLog.DPanicw
	Panicw = globalLog.Panicw
	Fatalw = globalLog.Fatalw
	Sync = globalLog.Sync
}
