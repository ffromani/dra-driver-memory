// this is taken from k8s.io/klog@a9f20c8ef6acb94a9ceca8d722a3e7f08fd4a574

/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package objref

import (
	"bytes"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
)

// ObjectRef references a kubernetes object
type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

func (ref ObjectRef) String() string {
	if ref.Namespace == "" {
		return ref.Name
	}
	var builder strings.Builder
	builder.Grow(len(ref.Namespace) + len(ref.Name) + 1)
	builder.WriteString(ref.Namespace)
	builder.WriteRune('/')
	builder.WriteString(ref.Name)
	return builder.String()
}

func (ref ObjectRef) WriteText(out *bytes.Buffer) {
	out.WriteRune('"')
	ref.writeUnquoted(out)
	out.WriteRune('"')
}

func (ref ObjectRef) writeUnquoted(out *bytes.Buffer) {
	if ref.Namespace != "" {
		out.WriteString(ref.Namespace)
		out.WriteRune('/')
	}
	out.WriteString(ref.Name)
}

// MarshalLog ensures that loggers with support for structured output will log
// as a struct by removing the String method via a custom type.
func (ref ObjectRef) MarshalLog() interface{} {
	type or ObjectRef
	return or(ref)
}

var _ logr.Marshaler = ObjectRef{}

// KMetadata is a subset of the kubernetes k8s.io/apimachinery/pkg/apis/meta/v1.Object interface
// this interface may expand in the future, but will always be a subset of the
// kubernetes k8s.io/apimachinery/pkg/apis/meta/v1.Object interface
type KMetadata interface {
	GetName() string
	GetNamespace() string
}

// KObj returns ObjectRef from ObjectMeta
func KObj(obj KMetadata) ObjectRef {
	if obj == nil {
		return ObjectRef{}
	}
	if val := reflect.ValueOf(obj); val.Kind() == reflect.Ptr && val.IsNil() {
		return ObjectRef{}
	}

	return ObjectRef{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}

// KRef returns ObjectRef from name and namespace
func KRef(namespace, name string) ObjectRef {
	return ObjectRef{
		Name:      name,
		Namespace: namespace,
	}
}
