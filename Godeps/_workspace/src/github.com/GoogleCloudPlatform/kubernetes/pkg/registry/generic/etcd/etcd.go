/*
Copyright 2014 Google Inc. All rights reserved.

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

package etcd

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeerr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/golang/glog"
)

// Etcd implements generic.Registry, backing it with etcd storage.
// It's intended to be embeddable, so that you can implement any
// non-generic functions if needed.
// You must supply a value for every field below before use; these are
// left public as it's meant to be overridable if need be.
// This object is intended to be copyable so that it can be used in
// different ways but share the same underlying behavior.
//
// The intended use of this type is embedding within a Kind specific
// RESTStorage implementation. This type provides CRUD semantics on
// a Kubelike resource, handling details like conflict detection with
// ResourceVersion and semantics. The RESTCreateStrategy and
// RESTUpdateStrategy are generic across all backends, and encapsulate
// logic specific to the API.
//
// TODO: make the default exposed methods exactly match a generic RESTStorage
type Etcd struct {
	// Called to make a new object, should return e.g., &api.Pod{}
	NewFunc func() runtime.Object

	// Called to make a new listing object, should return e.g., &api.PodList{}
	NewListFunc func() runtime.Object

	// Used for error reporting
	EndpointName string

	// Used for listing/watching; should not include trailing "/"
	KeyRootFunc func(ctx api.Context) string

	// Called for Create/Update/Get/Delete
	KeyFunc func(ctx api.Context, name string) (string, error)

	// Called to get the name of an object
	ObjectNameFunc func(obj runtime.Object) (string, error)

	// Return the TTL objects should be persisted with. Update is true if this
	// is an operation against an existing object.
	TTLFunc func(obj runtime.Object, update bool) (uint64, error)

	// Called on all objects returned from the underlying store, after
	// the exit hooks are invoked. Decorators are intended for integrations
	// that are above etcd and should only be used for specific cases where
	// storage of the value in etcd is not appropriate, since they cannot
	// be watched.
	Decorator rest.ObjectFunc
	// Allows extended behavior during creation
	CreateStrategy rest.RESTCreateStrategy
	// On create of an object, attempt to run a further operation.
	AfterCreate rest.ObjectFunc
	// Allows extended behavior during updates
	UpdateStrategy rest.RESTUpdateStrategy
	// On update of an object, attempt to run a further operation.
	AfterUpdate rest.ObjectFunc
	// If true, return the object that was deleted. Otherwise, return a generic
	// success status response.
	ReturnDeletedObject bool
	// On deletion of an object, attempt to run a further operation.
	AfterDelete rest.ObjectFunc

	// Used for all etcd access functions
	Helper tools.EtcdHelper
}

// NamespaceKeyRootFunc is the default function for constructing etcd paths to resource directories enforcing namespace rules.
func NamespaceKeyRootFunc(ctx api.Context, prefix string) string {
	key := prefix
	ns, ok := api.NamespaceFrom(ctx)
	if ok && len(ns) > 0 {
		key = key + "/" + ns
	}
	return key
}

// NamespaceKeyFunc is the default function for constructing etcd paths to a resource relative to prefix enforcing namespace rules.
// If no namespace is on context, it errors.
func NamespaceKeyFunc(ctx api.Context, prefix string, name string) (string, error) {
	key := NamespaceKeyRootFunc(ctx, prefix)
	ns, ok := api.NamespaceFrom(ctx)
	if !ok || len(ns) == 0 {
		return "", kubeerr.NewBadRequest("Namespace parameter required.")
	}
	if len(name) == 0 {
		return "", kubeerr.NewBadRequest("Name parameter required.")
	}
	key = key + "/" + name
	return key, nil
}

// List returns a list of all the items matching m.
// TODO: rename this to ListPredicate, take the default predicate function on the constructor, and
// introduce a List method that uses the default predicate function
func (e *Etcd) List(ctx api.Context, m generic.Matcher) (runtime.Object, error) {
	list := e.NewListFunc()
	err := e.Helper.ExtractToList(e.KeyRootFunc(ctx), list)
	if err != nil {
		return nil, err
	}
	return generic.FilterList(list, m, generic.DecoratorFunc(e.Decorator))
}

// CreateWithName inserts a new item with the provided name
// DEPRECATED: use Create instead
func (e *Etcd) CreateWithName(ctx api.Context, name string, obj runtime.Object) error {
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return err
	}
	if e.CreateStrategy != nil {
		if err := rest.BeforeCreate(e.CreateStrategy, ctx, obj); err != nil {
			return err
		}
	}
	ttl := uint64(0)
	if e.TTLFunc != nil {
		ttl, err = e.TTLFunc(obj, false)
		if err != nil {
			return err
		}
	}
	err = e.Helper.CreateObj(key, obj, ttl)
	err = etcderr.InterpretCreateError(err, e.EndpointName, name)
	if err == nil && e.Decorator != nil {
		err = e.Decorator(obj)
	}
	return err
}

// Create inserts a new item according to the unique key from the object.
func (e *Etcd) Create(ctx api.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(e.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}
	name, err := e.ObjectNameFunc(obj)
	if err != nil {
		return nil, err
	}
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	ttl := uint64(0)
	if e.TTLFunc != nil {
		ttl, err = e.TTLFunc(obj, false)
		if err != nil {
			return nil, err
		}
	}
	out := e.NewFunc()
	if err := e.Helper.Create(key, obj, out, ttl); err != nil {
		err = etcderr.InterpretCreateError(err, e.EndpointName, name)
		err = rest.CheckGeneratedNameError(e.CreateStrategy, err, obj)
		return nil, err
	}
	if e.AfterCreate != nil {
		if err := e.AfterCreate(out); err != nil {
			return nil, err
		}
	}
	if e.Decorator != nil {
		if err := e.Decorator(obj); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UpdateWithName updates the item with the provided name
// DEPRECATED: use Update instead
func (e *Etcd) UpdateWithName(ctx api.Context, name string, obj runtime.Object) error {
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return err
	}
	ttl := uint64(0)
	if e.TTLFunc != nil {
		ttl, err = e.TTLFunc(obj, false)
		if err != nil {
			return err
		}
	}
	err = e.Helper.SetObj(key, obj, ttl)
	err = etcderr.InterpretUpdateError(err, e.EndpointName, name)
	if err == nil && e.Decorator != nil {
		err = e.Decorator(obj)
	}
	return err
}

// Update performs an atomic update and set of the object. Returns the result of the update
// or an error. If the registry allows create-on-update, the create flow will be executed.
func (e *Etcd) Update(ctx api.Context, obj runtime.Object) (runtime.Object, bool, error) {
	name, err := e.ObjectNameFunc(obj)
	if err != nil {
		return nil, false, err
	}
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, false, err
	}
	creating := false
	out := e.NewFunc()
	err = e.Helper.AtomicUpdate(key, out, true, func(existing runtime.Object) (runtime.Object, error) {
		version, err := e.Helper.ResourceVersioner.ResourceVersion(existing)
		if err != nil {
			return nil, err
		}
		if version == 0 {
			if !e.UpdateStrategy.AllowCreateOnUpdate() {
				return nil, kubeerr.NewAlreadyExists(e.EndpointName, name)
			}
			creating = true
			if err := rest.BeforeCreate(e.CreateStrategy, ctx, obj); err != nil {
				return nil, err
			}
			return obj, nil
		}
		creating = false
		if err := rest.BeforeUpdate(e.UpdateStrategy, ctx, obj, existing); err != nil {
			return nil, err
		}
		// TODO: expose TTL
		return obj, nil
	})
	if err != nil {
		if creating {
			err = etcderr.InterpretCreateError(err, e.EndpointName, name)
			err = rest.CheckGeneratedNameError(e.CreateStrategy, err, obj)
		} else {
			err = etcderr.InterpretUpdateError(err, e.EndpointName, name)
		}
		return nil, false, err
	}
	if creating {
		if e.AfterCreate != nil {
			if err := e.AfterCreate(out); err != nil {
				return nil, false, err
			}
		}
	} else {
		if e.AfterUpdate != nil {
			if err := e.AfterUpdate(out); err != nil {
				return nil, false, err
			}
		}
	}
	if e.Decorator != nil {
		if err := e.Decorator(obj); err != nil {
			return nil, false, err
		}
	}
	return out, creating, nil
}

// Get retrieves the item from etcd.
func (e *Etcd) Get(ctx api.Context, name string) (runtime.Object, error) {
	obj := e.NewFunc()
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	err = e.Helper.ExtractObj(key, obj, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, e.EndpointName, name)
	}
	if e.Decorator != nil {
		if err := e.Decorator(obj); err != nil {
			return nil, err
		}
	}
	return obj, nil
}

// Delete removes the item from etcd.
func (e *Etcd) Delete(ctx api.Context, name string) (runtime.Object, error) {
	key, err := e.KeyFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	obj := e.NewFunc()
	if err := e.Helper.DeleteObj(key, obj); err != nil {
		return nil, etcderr.InterpretDeleteError(err, e.EndpointName, name)
	}
	if e.AfterDelete != nil {
		if err := e.AfterDelete(obj); err != nil {
			return nil, err
		}
	}
	if e.Decorator != nil {
		if err := e.Decorator(obj); err != nil {
			return nil, err
		}
	}
	if e.ReturnDeletedObject {
		return obj, nil
	}
	return &api.Status{Status: api.StatusSuccess}, nil
}

// Watch starts a watch for the items that m matches.
// TODO: Detect if m references a single object instead of a list.
// TODO: rename this to WatchPredicate, take the default predicate function on the constructor, and
// introduce a Watch method that uses the default predicate function
func (e *Etcd) Watch(ctx api.Context, m generic.Matcher, resourceVersion string) (watch.Interface, error) {
	version, err := tools.ParseWatchResourceVersion(resourceVersion, e.EndpointName)
	if err != nil {
		return nil, err
	}
	return e.Helper.WatchList(e.KeyRootFunc(ctx), version, func(obj runtime.Object) bool {
		matches, err := m.Matches(obj)
		if err != nil {
			glog.Errorf("unable to match watch: %v", err)
			return false
		}
		if matches && e.Decorator != nil {
			if err := e.Decorator(obj); err != nil {
				glog.Errorf("unable to decorate watch: %v", err)
				return false
			}
		}
		return matches
	})
}