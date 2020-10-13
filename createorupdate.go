package crutil

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func spewDiff(a, b interface{}) string {
	aStr := spew.Sdump(a)
	bStr := spew.Sdump(b)

	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(aStr, bStr, false)

	return dmp.DiffPrettyText(diffs)
}

func jsonDiff(a, b interface{}) string {
	aStr, _ := json.MarshalIndent(a, "", "  ")
	bStr, _ := json.MarshalIndent(b, "", "  ")

	dmp := diffmatchpatch.New()

	diffs := dmp.DiffMain(string(aStr), string(bStr), false)

	return dmp.DiffPrettyText(diffs)
}

type Object interface {
	metav1.Object
	runtime.Object
}

func mutate(f util.MutateFn, key client.ObjectKey, obj Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey, err := client.ObjectKeyFromObject(obj); err != nil || key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}

func CreateOrUpdate(ctx context.Context, c client.Client, obj Object, f util.MutateFn) (util.OperationResult, error) {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return util.OperationResultNone, err
	}

	if err := c.Get(ctx, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return util.OperationResultNone, err
		}
		if err := mutate(f, key, obj); err != nil {
			return util.OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return util.OperationResultNone, err
		}
		return util.OperationResultCreated, nil
	}

	existing := obj.DeepCopyObject()
	if err := mutate(f, key, obj); err != nil {
		return util.OperationResultNone, err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return util.OperationResultNone, nil
	}

	if err := c.Update(ctx, obj); err != nil {
		return util.OperationResultNone, err
	}

	fmt.Println(jsonDiff(existing, obj))

	return util.OperationResultUpdated, nil
}
