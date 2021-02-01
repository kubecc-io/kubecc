package rec

import (
	"errors"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/templates"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	MustExist ObjectCreator = func(ResolveContext) (client.Object, error) {
		return nil, errors.New("Object must already exist")
	}
)

func FromTemplate(templateName string) ObjectCreator {
	return func(rc ResolveContext) (client.Object, error) {
		tmplData, err := templates.Load(templateName, rc.Object)
		lg := lll.With(zap.Error(err), zap.String("name", templateName), zap.Any("spec", rc.Object))
		if err != nil {
			lg.Error("Error loading from template")
			return nil, err
		}
		obj, err := templates.Unmarshal(tmplData, rc.Client.Scheme())
		if err != nil {
			lg.Error("Error unmarshalling from template")
			return nil, err
		}
		obj.SetNamespace(rc.RootObject.GetNamespace())
		return obj, nil
	}
}
