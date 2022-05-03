/*
Copyright 2022 The Kubernetes Authors.

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

package logr

import (
	"github.com/go-logr/logr"
	"go.uber.org/atomic"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type KubeAwareLogSink struct {
	kubeAwareEnabled *atomic.Bool
	logger           logr.LogSink
}

func (k *KubeAwareLogSink) Init(info logr.RuntimeInfo) {
	k.logger.Init(info)
}

func (k *KubeAwareLogSink) Enabled(level int) bool {
	return k.logger.Enabled(level)
}

func (k *KubeAwareLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	if !k.KubeAwareEnabled() {
		k.logger.Info(level, msg, keysAndValues...)
		return
	}

	k.logger.Info(level, msg, k.wrapKeyAndValues(keysAndValues)...)
}

func (k *KubeAwareLogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	if !k.KubeAwareEnabled() {
		k.logger.Error(err, msg, keysAndValues...)
		return
	}
	k.logger.Error(err, msg, k.wrapKeyAndValues(keysAndValues)...)
}

func (k *KubeAwareLogSink) wrapKeyAndValues(keysAndValues []interface{}) []interface{} {
	result := make([]interface{}, len(keysAndValues))
	for i, item := range keysAndValues {
		if i%2 == 1 {
			// item is key, no need to resolve
			result[i] = item
			continue
		}
		switch val := item.(type) {
		case runtime.Object:
			result[i] = kubeObjectWrapper{obj: val}
		case types.NamespacedName:
			result[i] = namespacedNameWrapper{NamespacedName: val}
		default:
			result[i] = item
		}
	}
	return result
}

func (k *KubeAwareLogSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &KubeAwareLogSink{
		kubeAwareEnabled: k.kubeAwareEnabled,
		logger:           k.logger.WithValues(keysAndValues...),
	}
}

func (k *KubeAwareLogSink) WithName(name string) logr.LogSink {
	return &KubeAwareLogSink{
		kubeAwareEnabled: k.kubeAwareEnabled,
		logger:           k.logger.WithName(name),
	}
}

func (k *KubeAwareLogSink) KubeAwareEnabled() bool {
	return k.kubeAwareEnabled.Load()
}

func (k *KubeAwareLogSink) SetKubeAwareEnabled(enabled bool) {
	k.kubeAwareEnabled.Store(enabled)
}