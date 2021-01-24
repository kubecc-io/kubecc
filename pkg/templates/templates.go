package templates

import (
	"bytes"
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type wrapper struct {
	Spec interface{}
}

func Load(name string, spec interface{}) ([]byte, error) {
	tmpl := template.New(name).Funcs(Funcs())
	tmpl, err := tmpl.ParseFiles(fmt.Sprintf("/templates/%s", name))
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
	return out.(client.Object), err
}
