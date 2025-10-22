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

package command

import (
	"fmt"

	"github.com/go-logr/logr"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/ffromani/dra-driver-memory/pkg/driver"
	"github.com/ffromani/dra-driver-memory/pkg/unitconv"
)

func MakeManifests(params Params, setupLogger logr.Logger) error {
	machine, err := GetMachineData(params)
	if err != nil {
		return err
	}
	hpSizes := sets.New[uint64]()
	for _, zone := range machine.Zones {
		if zone.Memory == nil {
			continue
		}
		hpSizes.Insert(zone.Memory.SupportedPageSizes...)
	}
	celExpr := fmt.Sprintf("device.driver == %q", driver.Name)
	devClasses := []resourceapi.DeviceClass{}
	devClasses = append(devClasses, resourceapi.DeviceClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "resource.k8s.io/v1",
			Kind:       "DeviceClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "dra.memory",
		},
		Spec: resourceapi.DeviceClassSpec{
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: celExpr,
					},
				},
			},
		},
	})
	for _, hpSize := range sets.List(hpSizes) {
		count, hpUnit := unitconv.NarrowSize(hpSize)
		hpName := fmt.Sprintf("hugepages-%d%s", count, unitconv.Minimize(hpUnit))
		devClasses = append(devClasses, resourceapi.DeviceClass{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "resource.k8s.io/v1",
				Kind:       "DeviceClass",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "dra." + hpName,
			},
			Spec: resourceapi.DeviceClassSpec{
				Selectors: []resourceapi.DeviceSelector{
					{
						CEL: &resourceapi.CELDeviceSelector{
							Expression: celExpr,
						},
					},
				},
			},
		})
	}
	for _, devClass := range devClasses {
		fmt.Println("---")
		printYAML(devClass, setupLogger)
	}
	return nil
}
