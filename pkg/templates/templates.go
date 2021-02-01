package templates

import (
	"bytes"
	"path"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type wrapper struct {
	Spec interface{}
}

var pathPrefix = "/templates"

func SetPathPrefix(prefix string) {
	pathPrefix = prefix
}

func Load(name string, spec interface{}) ([]byte, error) {
	tmpl := template.New(name).Funcs(Funcs())
	tmpl, err := tmpl.ParseFiles(path.Join(pathPrefix, name))
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, wrapper{Spec: spec})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Unmarshal(data []byte, scheme *runtime.Scheme) (client.Object, error) {
	ds := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	out, _, err := ds.Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	return out.(client.Object), nil
}