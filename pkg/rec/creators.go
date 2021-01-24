package rec

import (
	"errors"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	MustExist ObjectCreator = func(ResolveContext) (client.Object, error) {
		return nil, errors.New("Object must already exist")
	}
)

func FromTemplate(templateName string, rc ResolveContext) ObjectCreator {
	return func(rc ResolveContext) (client.Object, error) {
		agent := rc.Object.(*v1alpha1.AgentSpec)
		tmplData, err := templates.Load("agent_daemonset", agent)
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
