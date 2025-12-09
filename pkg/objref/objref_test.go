/*
 * Copyright 2025 The Kubernetes Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package objref

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	type testcase struct {
		name string
		obj  ObjectRef
		exp  string
	}

	testcases := []testcase{
		{
			name: "empty",
		},
		{
			name: "name only",
			obj: ObjectRef{
				Name: "foo",
			},
			exp: "foo",
		},
		{
			name: "namespace only",
			obj: ObjectRef{
				Namespace: "foo",
			},
			exp: "foo/",
		},
		{
			name: "fully specified",
			obj: ObjectRef{
				Namespace: "foo",
				Name:      "bar",
			},
			exp: "foo/bar",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			got := tcase.obj.String()
			require.Equal(t, got, tcase.exp)
		})
	}
}

func TestKRefToString(t *testing.T) {
	type testcase struct {
		desc      string
		namespace string
		name      string
		exp       string
	}

	testcases := []testcase{
		{
			desc: "empty",
		},
		{
			desc: "name only",
			name: "foo",
			exp:  "foo",
		},
		{
			desc:      "namespace only",
			namespace: "foo",
			exp:       "foo/",
		},
		{
			desc:      "fully specified",
			namespace: "foo",
			name:      "bar",
			exp:       "foo/bar",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.name, func(t *testing.T) {
			obj := KRef(tcase.namespace, tcase.name)
			got := obj.String()
			require.Equal(t, got, tcase.exp)
		})
	}
}

func TestKObjToString(t *testing.T) {
	type testcase struct {
		desc string
		md   KMetadata
		exp  string
	}

	var ptr *fakeObject
	testcases := []testcase{
		{
			desc: "nil",
		},
		{
			desc: "empty",
			md:   &fakeObject{},
		},
		{
			desc: "nil ptr",
			md:   ptr,
		},
		{
			desc: "name only",
			md: &fakeObject{
				Name: "foo",
			},
			exp: "foo",
		},
		{
			desc: "namespace only",
			md: &fakeObject{
				Namespace: "foo",
			},
			exp: "foo/",
		},
		{
			desc: "fully specified",
			md: &fakeObject{
				Namespace: "foo",
				Name:      "bar",
			},
			exp: "foo/bar",
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.desc, func(t *testing.T) {
			obj := KObj(tcase.md)
			got := obj.String()
			require.Equal(t, got, tcase.exp)
		})
	}
}

func TestKObjWithWriteText(t *testing.T) {
	type testcase struct {
		desc string
		md   KMetadata
		exp  string
	}

	var ptr *fakeObject
	testcases := []testcase{
		{
			desc: "nil",
			exp:  `""`,
		},
		{
			desc: "nil ptr",
			md:   ptr,
			exp:  `""`,
		},
		{
			desc: "empty",
			md:   &fakeObject{},
			exp:  `""`,
		},
		{
			desc: "name only",
			md: &fakeObject{
				Name: "foo",
			},
			exp: `"foo"`,
		},
		{
			desc: "namespace only",
			md: &fakeObject{
				Namespace: "foo",
			},
			exp: `"foo/"`,
		},
		{
			desc: "fully specified",
			md: &fakeObject{
				Namespace: "foo",
				Name:      "bar",
			},
			exp: `"foo/bar"`,
		},
	}

	for _, tcase := range testcases {
		t.Run(tcase.desc, func(t *testing.T) {
			obj := KObj(tcase.md)
			buf := bytes.Buffer{}
			obj.WriteText(&buf)
			got := buf.String()
			require.Equal(t, got, tcase.exp)
		})
	}
}

type fakeObject struct {
	Namespace string
	Name      string
}

func (fo *fakeObject) GetNamespace() string {
	return fo.Namespace
}

func (fo *fakeObject) GetName() string {
	return fo.Name
}
