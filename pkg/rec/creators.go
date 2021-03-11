package rec

import (
	"embed"
	_ "embed"

	"github.com/cobalt77/kubecc/pkg/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed objects
var objectsFS embed.FS

type mustExist struct {
	error
}

func (e mustExist) Error() string {
	return "Object must already exist"
}

var (
	MustExist ObjectCreator = func(ResolveContext) (client.Object, error) {
		return nil, &mustExist{}
	}
)

func FromTemplate(templateName string) ObjectCreator {
	return func(rc ResolveContext) (client.Object, error) {
		tmplData, err := templates.Load(objectsFS, templateName, rc.Object)
		if err != nil {
			return nil, err
		}
		obj, err := templates.Unmarshal(tmplData, rc.Client.Scheme())
		if err != nil {
			return nil, err
		}
		obj.SetNamespace(rc.RootObject.GetNamespace())
		return obj, nil
	}
}
