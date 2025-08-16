// Copyright 2025 Marc Siegenthaler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csi

import (
	"context"

	"k8s.io/klog/v2"
)

func LogFromContext(ctx context.Context) klog.Logger {
	return klog.FromContext(ctx)
}

func LogAndExpandContext(ctx context.Context, keysandvalues ...any) (klog.Logger, context.Context) {
	log := klog.FromContext(ctx).WithValues(keysandvalues...)
	ctx = klog.NewContext(ctx, log)
	return log, ctx
}
