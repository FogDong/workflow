/*
Copyright 2021 The KubeVela Authors.

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

package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
)

const (
	// EnableSuspendOnFailure enable suspend on workflow failure
	EnableSuspendOnFailure featuregate.Feature = "EnableSuspendOnFailure"
	// EnablePersistWorkflowRecord enable persist workflow record
	EnablePersistWorkflowRecord featuregate.Feature = "EnablePersistWorkflowRecord"
)

var defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	EnableSuspendOnFailure:      {Default: false, PreRelease: featuregate.Alpha},
	EnablePersistWorkflowRecord: {Default: true, PreRelease: featuregate.Alpha},
}

func init() {
	runtime.Must(feature.DefaultMutableFeatureGate.Add(defaultFeatureGates))
}
