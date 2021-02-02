package scheduler

// type HandlerFunc func(*types.CompileStatus)

// type CompileTask struct {
// 	stream   types.Agent_CompileClient
// 	statusCh chan *types.CompileStatus
// 	errCh    chan error
// }

// func (t *CompileTask) Status() <-chan *types.CompileStatus {
// 	return t.statusCh
// }

// func (t *CompileTask) Error() <-chan error {
// 	return t.errCh
// }

// func (t *CompileTask) Canceled() <-chan struct{} {
// 	return t.stream.Context().Done()
// }

// func (t *CompileTask) Context() context.Context {
// 	return t.stream.Context()
// }

// func (t *CompileTask) Start() {
// 	go func() {
// 		defer close(t.statusCh)
// 		defer close(t.errCh)
// 		for {
// 			status, err := t.stream.Recv()
// 			if err == io.EOF {
// 				t.errCh <- err
// 				return
// 			} else if err != nil {
// 				t.errCh <- err
// 				return
// 			}
// 			t.statusCh <- status
// 			if status.CompileStatus == types.CompileStatus_Fail ||
// 				status.CompileStatus == types.CompileStatus_Success {
// 				return
// 			}
// 		}
// 	}()
// }

// func NewCompileTask(
// 	stream types.Agent_CompileClient,
// ) *CompileTask {
// 	return &CompileTask{
// 		statusCh: make(chan *types.CompileStatus),
// 		errCh:    make(chan error),
// 		stream:   stream,
// 	}
// }
