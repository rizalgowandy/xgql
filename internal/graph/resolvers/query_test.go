package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var _ generated.QueryResolver = &query{}

func TestQueryKubernetesResource(t *testing.T) {
	errBoom := errors.New("boom")

	gkr, _ := model.GetKubernetesResource(&unstructured.Unstructured{})

	type args struct {
		ctx context.Context
		id  model.ReferenceID
	}
	type want struct {
		kr   model.KubernetesResource
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"GetKubernetesResourceError": {
			reason: "If we can't get the resource we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetResource).Error()),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the resource  we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				kr: gkr,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.KubernetesResource(tc.args.ctx, tc.args.id)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.KubernetesResource(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.KubernetesResource(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.kr, got, cmpopts.IgnoreFields(model.GenericResource{}, "Raw"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.KubernetesResource(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryKubernetesResources(t *testing.T) {
	errBoom := errors.New("boom")

	kr := unstructured.Unstructured{}
	gkr, _ := model.GetKubernetesResource(&kr)

	group := "example.org"
	version := "v1"
	apiVersion := schema.GroupVersion{Group: group, Version: version}.String()
	kind := "Example"

	// In almost all real cases this would be 'ExampleList', but we infer that
	// when ListKind is not set, and want to test that this will override it.
	listKind := "Examples"

	ns := "default"

	type args struct {
		ctx        context.Context
		apiVersion string
		kind       string
		listKind   *string
		namespace  *string
	}
	type want struct {
		krc  *model.KubernetesResourceConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListKubernetesResourcesError": {
			reason: "If we can't list defined claims we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListResources).Error()),
				},
			},
		},
		"GVKOnly": {
			reason: "We should successfully return any Kubernetes resources of the specified GVK that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						u := *obj.(*unstructured.UnstructuredList)

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: kind + "List"}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{kr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:        graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				apiVersion: apiVersion,
				kind:       kind,
			},
			want: want{
				krc: &model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{gkr},
					TotalCount: 1,
				},
			},
		},
		"WithListKind": {
			reason: "We should successfully list, model, and return resources of a bespoke listKind.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						u := *obj.(*unstructured.UnstructuredList)

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: listKind}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{kr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:        graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				apiVersion: apiVersion,
				kind:       kind,
				listKind:   &listKind,
			},
			want: want{
				krc: &model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{gkr},
					TotalCount: 1,
				},
			},
		},
		"WithNamespace": {
			reason: "We should successfully list, model, and return resources from within a specific namespace.",
			clients: ClientCacheFn(func(_ auth.Credentials, o ...clients.GetOption) (client.Client, error) {
				if len(o) != 1 {
					t.Errorf("Expected 1 GetOption, got %d", len(o))
				}
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						u := *obj.(*unstructured.UnstructuredList)

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: kind + "List"}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{kr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:        graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				apiVersion: apiVersion,
				kind:       kind,
				namespace:  &ns,
			},
			want: want{
				krc: &model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{gkr},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.KubernetesResources(tc.args.ctx, tc.args.apiVersion, tc.args.kind, tc.args.listKind, tc.args.namespace)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.krc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQuerySecret(t *testing.T) {
	errBoom := errors.New("boom")

	gsec := model.GetSecret(&corev1.Secret{})

	type args struct {
		ctx       context.Context
		namespace string
		name      string
	}
	type want struct {
		sec  *model.Secret
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"GetSecretError": {
			reason: "If we can't get the secret we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetSecret).Error()),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the secret we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				sec: &gsec,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Secret(tc.args.ctx, tc.args.namespace, tc.args.name)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Secret(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Secret(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.sec, got, cmp.AllowUnexported(model.Secret{}), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.Secret(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryConfigMap(t *testing.T) {
	errBoom := errors.New("boom")

	gsec := model.GetConfigMap(&corev1.ConfigMap{})

	type args struct {
		ctx       context.Context
		namespace string
		name      string
	}
	type want struct {
		cm   *model.ConfigMap
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"GetConfigMapError": {
			reason: "If we can't get the config map we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetConfigMap).Error()),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the config map we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				cm: &gsec,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.ConfigMap(tc.args.ctx, tc.args.namespace, tc.args.name)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ConfigMap(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ConfigMap(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cm, got, cmp.AllowUnexported(model.ConfigMap{}), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.ConfigMap(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryProviders(t *testing.T) {
	errBoom := errors.New("boom")

	p := pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "coolprovider"}}
	gp := model.GetProvider(&p)

	type args struct {
		ctx context.Context
	}
	type want struct {
		pc   *model.ProviderConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListProvidersError": {
			reason: "If we can't list providers we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListProviders).Error()),
				},
			},
		},
		"Success": {
			reason: "We should successfully return any providers we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderList) = pkgv1.ProviderList{Items: []pkgv1.Provider{p}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				pc: &model.ProviderConnection{
					Nodes:      []model.Provider{gp},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Providers(tc.args.ctx)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Providers(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Providers(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Providers(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryProviderRevisions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ProviderGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ProviderKind,
		Name:       "coolprovider",
	}

	// The active ProviderRevision that we control.
	active := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "coolrev",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			}},
		},
		Spec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionActive},
	}
	gactive := model.GetProviderRevision(&active)

	// A ProviderRevision we control, but that is inactive.
	inactive := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inactiverev",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			}},
		},
		Spec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionInactive},
	}
	ginactive := model.GetProviderRevision(&inactive)

	// A ProviderRevision which we do not control.
	other := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-ours",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "other",
			}},
		},
	}
	gother := model.GetProviderRevision(&other)

	type args struct {
		ctx    context.Context
		id     *model.ReferenceID
		active *bool
	}
	type want struct {
		pc   *model.ProviderRevisionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListRevisionsError": {
			reason: "If we can't list revisions we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListProviderRevs).Error()),
				},
			},
		},
		"AllRevisions": {
			reason: "We should successfully return any revisions we own that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderRevisionList) = pkgv1.ProviderRevisionList{
							Items: []pkgv1.ProviderRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				pc: &model.ProviderRevisionConnection{
					Nodes:      []model.ProviderRevision{gother, gactive, ginactive},
					TotalCount: 3,
				},
			},
		},
		"ProvidersRevisions": {
			reason: "We should successfully return any revisions the supplied provider id owns that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderRevisionList) = pkgv1.ProviderRevisionList{
							Items: []pkgv1.ProviderRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				id:  &id,
			},
			want: want{
				pc: &model.ProviderRevisionConnection{
					Nodes:      []model.ProviderRevision{gactive, ginactive},
					TotalCount: 2,
				},
			},
		},
		"ActiveRevisions": {
			reason: "We should successfully return any active revisions that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderRevisionList) = pkgv1.ProviderRevisionList{
							Items: []pkgv1.ProviderRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:    graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				active: pointer.BoolPtr(true),
			},
			want: want{
				pc: &model.ProviderRevisionConnection{
					Nodes:      []model.ProviderRevision{gactive},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.ProviderRevisions(tc.args.ctx, tc.args.id, tc.args.active)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryCustomResourceDefinitions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ConfigurationRevisionKind,
		Name:       "example",
	}

	owned := kextv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
		Name: "coolconfig",
		OwnerReferences: []metav1.OwnerReference{
			// Some spurious owner references that we should ignore.
			{
				APIVersion: "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "wat",
			},
			// The reference that indicates this XRD is owned by our desired
			// ConfigurationRevision (or a ConfigurationRevision generally).
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			},
		},
	}}
	gowned := model.GetCustomResourceDefinition(&owned)

	dangler := kextv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gdangler := model.GetCustomResourceDefinition(&dangler)

	type args struct {
		ctx      context.Context
		revision *model.ReferenceID
	}
	type want struct {
		xrdc *model.CustomResourceDefinitionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListCRDsError": {
			reason: "If we can't list CRDs we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"AllCRDs": {
			reason: "We should successfully return all CRDs we can list and model when no arguments are supplied.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*kextv1.CustomResourceDefinitionList) = kextv1.CustomResourceDefinitionList{
							Items: []kextv1.CustomResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				xrdc: &model.CustomResourceDefinitionConnection{
					Nodes: []model.CustomResourceDefinition{
						gdangler,
						gowned,
					},
					TotalCount: 2,
				},
			},
		},
		"OwnedCRDs": {
			reason: "We should successfully return the CRDs we can list and model that are owned by the supplied ID.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*kextv1.CustomResourceDefinitionList) = kextv1.CustomResourceDefinitionList{
							Items: []kextv1.CustomResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				revision: &id,
			},
			want: want{
				xrdc: &model.CustomResourceDefinitionConnection{
					Nodes: []model.CustomResourceDefinition{
						gowned,
					},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.CustomResourceDefinitions(tc.args.ctx, tc.args.revision)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xrdc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryConfigurations(t *testing.T) {
	errBoom := errors.New("boom")

	c := pkgv1.Configuration{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gc := model.GetConfiguration(&c)

	type args struct {
		ctx context.Context
	}
	type want struct {
		cc   *model.ConfigurationConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListConfigurationsError": {
			reason: "If we can't list configurations we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"Success": {
			reason: "We should successfully return any configurations we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationList) = pkgv1.ConfigurationList{Items: []pkgv1.Configuration{c}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				cc: &model.ConfigurationConnection{
					Nodes:      []model.Configuration{gc},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Configurations(tc.args.ctx)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryConfigurationRevisions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ConfigurationGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ConfigurationKind,
		Name:       "coolconfig",
	}

	// The active ConfigurationRevision that we control.
	active := pkgv1.ConfigurationRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "coolrev",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			}},
		},
		Spec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionActive},
	}
	gactive := model.GetConfigurationRevision(&active)

	// A ConfigurationRevision we control, but that is inactive.
	inactive := pkgv1.ConfigurationRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inactiverev",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			}},
		},
		Spec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionInactive},
	}
	ginactive := model.GetConfigurationRevision(&inactive)

	// A ConfigurationRevision which we do not control.
	other := pkgv1.ConfigurationRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-ours",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "other",
			}},
		},
	}
	gother := model.GetConfigurationRevision(&other)

	type args struct {
		ctx    context.Context
		id     *model.ReferenceID
		active *bool
	}
	type want struct {
		pc   *model.ConfigurationRevisionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListRevisionsError": {
			reason: "If we can't list revisions we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigRevs).Error()),
				},
			},
		},
		"AllRevisions": {
			reason: "We should successfully return any revisions we own that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationRevisionList) = pkgv1.ConfigurationRevisionList{
							Items: []pkgv1.ConfigurationRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				pc: &model.ConfigurationRevisionConnection{
					Nodes:      []model.ConfigurationRevision{gother, gactive, ginactive},
					TotalCount: 3,
				},
			},
		},
		"ConfigurationsRevisions": {
			reason: "We should successfully return any revisions the supplied provider id owns that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationRevisionList) = pkgv1.ConfigurationRevisionList{
							Items: []pkgv1.ConfigurationRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				id:  &id,
			},
			want: want{
				pc: &model.ConfigurationRevisionConnection{
					Nodes:      []model.ConfigurationRevision{gactive, ginactive},
					TotalCount: 2,
				},
			},
		},
		"ActiveRevisions": {
			reason: "We should successfully return any active revisions that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationRevisionList) = pkgv1.ConfigurationRevisionList{
							Items: []pkgv1.ConfigurationRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:    graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				active: pointer.BoolPtr(true),
			},
			want: want{
				pc: &model.ConfigurationRevisionConnection{
					Nodes:      []model.ConfigurationRevision{gactive},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.ConfigurationRevisions(tc.args.ctx, tc.args.id, tc.args.active)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryCompositeResourceDefinitions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ConfigurationRevisionKind,
		Name:       "example",
	}

	owned := extv1.CompositeResourceDefinition{ObjectMeta: metav1.ObjectMeta{
		Name: "coolconfig",
		OwnerReferences: []metav1.OwnerReference{
			// Some spurious owner references that we should ignore.
			{
				APIVersion: "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "wat",
			},
			// The reference that indicates this XRD is owned by our desired
			// ConfigurationRevision (or a ConfigurationRevision generally).
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			},
		},
	}}
	gowned := model.GetCompositeResourceDefinition(&owned)

	dangler := extv1.CompositeResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gdangler := model.GetCompositeResourceDefinition(&dangler)

	type args struct {
		ctx      context.Context
		revision *model.ReferenceID
		dangling *bool
	}
	type want struct {
		xrdc *model.CompositeResourceDefinitionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListXRDsError": {
			reason: "If we can't list XRDs we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"AllXRDs": {
			reason: "We should successfully return all XRDs we can list and model when no arguments are supplied.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				xrdc: &model.CompositeResourceDefinitionConnection{
					Nodes: []model.CompositeResourceDefinition{
						gdangler,
						gowned,
					},
					TotalCount: 2,
				},
			},
		},
		"DanglingXRDs": {
			reason: "We should successfully return dangling XRDs we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				dangling: pointer.BoolPtr(true),
			},
			want: want{
				xrdc: &model.CompositeResourceDefinitionConnection{
					Nodes: []model.CompositeResourceDefinition{
						gdangler,
					},
					TotalCount: 1,
				},
			},
		},
		"OwnedXRDs": {
			reason: "We should successfully return the XRDs we can list and model that are owned by the supplied ID.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				revision: &id,
			},
			want: want{
				xrdc: &model.CompositeResourceDefinitionConnection{
					Nodes: []model.CompositeResourceDefinition{
						gowned,
					},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.CompositeResourceDefinitions(tc.args.ctx, tc.args.revision, tc.args.dangling)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xrdc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryCompositions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ConfigurationRevisionKind,
		Name:       "example",
	}

	owned := extv1.Composition{ObjectMeta: metav1.ObjectMeta{
		Name: "coolconfig",
		OwnerReferences: []metav1.OwnerReference{
			// Some spurious owner references that we should ignore.
			{
				APIVersion: "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "wat",
			},
			// The reference that indicates this Composition is owned by our
			// desired ConfigurationRevision (or a ConfigurationRevision
			// generally).
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			},
		},
	}}
	gowned := model.GetComposition(&owned)

	dangler := extv1.Composition{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gdangler := model.GetComposition(&dangler)

	type args struct {
		ctx      context.Context
		revision *model.ReferenceID
		dangling *bool
	}
	type want struct {
		cc   *model.CompositionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListCompositionsError": {
			reason: "If we can't list compositions we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"AllCompositions": {
			reason: "We should successfully return all compositions we can list and model when no arguments are supplied.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositionList) = extv1.CompositionList{
							Items: []extv1.Composition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				cc: &model.CompositionConnection{
					Nodes: []model.Composition{
						gdangler,
						gowned,
					},
					TotalCount: 2,
				},
			},
		},
		"DanglingCompositions": {
			reason: "We should successfully return dangling compositions we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositionList) = extv1.CompositionList{
							Items: []extv1.Composition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				dangling: pointer.BoolPtr(true),
			},
			want: want{
				cc: &model.CompositionConnection{
					Nodes: []model.Composition{
						gdangler,
					},
					TotalCount: 1,
				},
			},
		},
		"OwnedCompositions": {
			reason: "We should successfully return the compositions we can list and model that are owned by the supplied ID.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositionList) = extv1.CompositionList{
							Items: []extv1.Composition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				revision: &id,
			},
			want: want{
				cc: &model.CompositionConnection{
					Nodes: []model.Composition{
						gowned,
					},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Compositions(tc.args.ctx, tc.args.revision, tc.args.dangling)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
