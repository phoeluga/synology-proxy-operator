// Package controller implements the SynologyReverseProxy reconciler.
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/synology"
)

const (
	finalizerName       = "proxy.hnet.io/finalizer"
	credentialsSecret   = "synology-credentials"
	conditionTypeReady  = "Ready"
	conditionTypeSynced = "Synced"
)

// SynologyReverseProxyReconciler reconciles SynologyReverseProxy objects.
type SynologyReverseProxyReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	OperatorNamespace string
}

// +kubebuilder:rbac:groups=proxy.hnet.io,resources=synologyreverseproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=proxy.hnet.io,resources=synologyreverseproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=proxy.hnet.io,resources=synologyreverseproxies/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is the main reconciliation loop.
func (r *SynologyReverseProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	srp := &proxyv1alpha1.SynologyReverseProxy{}
	if err := r.Get(ctx, req.NamespacedName, srp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Build Synology client from credentials secret.
	synClient, err := r.buildSynologyClient(ctx)
	if err != nil {
		logger.Error(err, "failed to build Synology client")
		r.setCondition(srp, conditionTypeReady, metav1.ConditionFalse, "CredentialsError", err.Error())
		_ = r.Status().Update(ctx, srp)
		return ctrl.Result{}, err
	}

	if err := synClient.Login(); err != nil {
		logger.Error(err, "failed to login to Synology API")
		r.setCondition(srp, conditionTypeReady, metav1.ConditionFalse, "LoginFailed", err.Error())
		_ = r.Status().Update(ctx, srp)
		return ctrl.Result{}, err
	}
	defer func() {
		if err := synClient.Logout(); err != nil {
			logger.Error(err, "failed to logout from Synology API")
		}
	}()

	// Handle deletion via finalizer.
	if !srp.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, srp, synClient)
	}

	// Ensure finalizer is present.
	if !controllerutil.ContainsFinalizer(srp, finalizerName) {
		controllerutil.AddFinalizer(srp, finalizerName)
		if err := r.Update(ctx, srp); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.reconcileUpsert(ctx, srp, synClient)
}

// reconcileUpsert creates or updates the reverse proxy record on Synology.
func (r *SynologyReverseProxyReconciler) reconcileUpsert(
	ctx context.Context,
	srp *proxyv1alpha1.SynologyReverseProxy,
	synClient *synology.Client,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	spec := srp.Spec

	// Resolve optional ACL profile name → ID.
	aclProfileID := ""
	if spec.ACLProfile != "" {
		id, err := synClient.GetACLProfileID(spec.ACLProfile)
		if err != nil {
			logger.Error(err, "failed to resolve ACL profile", "profile", spec.ACLProfile)
			r.setCondition(srp, conditionTypeSynced, metav1.ConditionFalse, "ACLProfileError", err.Error())
			_ = r.Status().Update(ctx, srp)
			return ctrl.Result{}, err
		}
		if id == "" {
			msg := fmt.Sprintf("ACL profile %q not found on Synology", spec.ACLProfile)
			logger.Info(msg)
			r.setCondition(srp, conditionTypeSynced, metav1.ConditionFalse, "ACLProfileNotFound", msg)
			_ = r.Status().Update(ctx, srp)
			return ctrl.Result{}, fmt.Errorf(msg)
		}
		aclProfileID = id
	}

	proxySpec := synology.ReverseProxySpec{
		Description:    spec.Description,
		SourceScheme:   spec.SourceProtocol,
		SourceHostname: spec.SourceHostname,
		SourcePort:     spec.SourcePort,
		DestScheme:     spec.DestProtocol,
		DestHostname:   spec.DestHostname,
		DestPort:       spec.DestPort,
		ACLProfileID:   aclProfileID,
	}

	uuid, err := synClient.UpsertReverseProxy(proxySpec)
	if err != nil {
		logger.Error(err, "failed to upsert reverse proxy record")
		r.setCondition(srp, conditionTypeSynced, metav1.ConditionFalse, "UpsertFailed", err.Error())
		_ = r.Status().Update(ctx, srp)
		return ctrl.Result{}, err
	}

	logger.Info("reverse proxy record upserted", "uuid", uuid)
	srp.Status.UUID = uuid

	// Optionally assign a matching wildcard certificate.
	if spec.AssignCertificate {
		if err := synClient.AssignCertificate(uuid, spec.SourceHostname); err != nil {
			logger.Error(err, "failed to assign certificate", "hostname", spec.SourceHostname)
			r.setCondition(srp, conditionTypeSynced, metav1.ConditionFalse, "CertAssignFailed", err.Error())
			_ = r.Status().Update(ctx, srp)
			return ctrl.Result{}, err
		}

		certID, _, err := synClient.FindMatchingCert(spec.SourceHostname)
		if err != nil {
			logger.Error(err, "failed to find matching cert after assignment")
		} else {
			srp.Status.CertID = certID
		}
		logger.Info("certificate assigned", "certID", srp.Status.CertID)
	}

	r.setCondition(srp, conditionTypeReady, metav1.ConditionTrue, "Reconciled", "Reverse proxy record is in sync")
	r.setCondition(srp, conditionTypeSynced, metav1.ConditionTrue, "Synced", "Successfully synced with Synology")

	if err := r.Status().Update(ctx, srp); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileDelete removes the reverse proxy record from Synology and strips the finalizer.
func (r *SynologyReverseProxyReconciler) reconcileDelete(
	ctx context.Context,
	srp *proxyv1alpha1.SynologyReverseProxy,
	synClient *synology.Client,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(srp, finalizerName) {
		if err := synClient.DeleteRecord(srp.Spec.Description); err != nil {
			logger.Error(err, "failed to delete reverse proxy record from Synology")
			r.setCondition(srp, conditionTypeSynced, metav1.ConditionFalse, "DeleteFailed", err.Error())
			_ = r.Status().Update(ctx, srp)
			return ctrl.Result{}, err
		}
		logger.Info("reverse proxy record deleted from Synology", "description", srp.Spec.Description)

		controllerutil.RemoveFinalizer(srp, finalizerName)
		if err := r.Update(ctx, srp); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// buildSynologyClient reads the credentials secret and constructs a Client.
func (r *SynologyReverseProxyReconciler) buildSynologyClient(ctx context.Context) (*synology.Client, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      credentialsSecret,
		Namespace: r.OperatorNamespace,
	}
	if err := r.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("failed to get credentials secret %q: %w", credentialsSecret, err)
	}

	rawURL := string(secret.Data["url"])
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	if rawURL == "" || username == "" || password == "" {
		return nil, fmt.Errorf("credentials secret %q must contain keys: url, username, password", credentialsSecret)
	}

	skipTLS := string(secret.Data["skipTLSVerify"]) == "true"
	return synology.NewClient(rawURL, username, password, skipTLS), nil
}

// setCondition is a helper that upserts a metav1.Condition on the status.
func (r *SynologyReverseProxyReconciler) setCondition(
	srp *proxyv1alpha1.SynologyReverseProxy,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	meta.SetStatusCondition(&srp.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: srp.Generation,
	})
}

// SetupWithManager registers the controller with the manager.
func (r *SynologyReverseProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&proxyv1alpha1.SynologyReverseProxy{}).
		Complete(r)
}
