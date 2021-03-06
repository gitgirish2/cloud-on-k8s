// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package controller

import (
	"fmt"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	beatv1beta1 "github.com/elastic/cloud-on-k8s/pkg/apis/beat/v1beta1"
	commonv1 "github.com/elastic/cloud-on-k8s/pkg/apis/common/v1"
	esv1 "github.com/elastic/cloud-on-k8s/pkg/apis/elasticsearch/v1"
	"github.com/elastic/cloud-on-k8s/pkg/controller/association"
	"github.com/elastic/cloud-on-k8s/pkg/controller/common/operator"
	"github.com/elastic/cloud-on-k8s/pkg/controller/common/version"
	eslabel "github.com/elastic/cloud-on-k8s/pkg/controller/elasticsearch/label"
	esuser "github.com/elastic/cloud-on-k8s/pkg/controller/elasticsearch/user"
	"github.com/elastic/cloud-on-k8s/pkg/utils/k8s"
	"github.com/elastic/cloud-on-k8s/pkg/utils/rbac"
)

const (
	// BeatAssociationLabelName marks resources created by this controller for easier retrieval.
	BeatAssociationLabelName = "beatassociation.k8s.elastic.co/name"
	// BeatAssociationLabelNamespace marks resources created by this controller for easier retrieval.
	BeatAssociationLabelNamespace = "beatassociation.k8s.elastic.co/namespace"
	// BeatAssociationLabelType marks the type of association
	BeatAssociationLabelType = "beatassociation.k8s.elastic.co/type"
)

func AddBeatES(mgr manager.Manager, accessReviewer rbac.AccessReviewer, params operator.Parameters) error {
	return association.AddAssociationController(mgr, accessReviewer, params, association.AssociationInfo{
		AssociationObjTemplate: func() commonv1.Association { return &beatv1beta1.BeatESAssociation{} },
		ElasticsearchRef: func(c k8s.Client, association commonv1.Association) (bool, commonv1.ObjectSelector, error) {
			return true, association.AssociationRef(), nil
		},
		ExternalServiceURL:  getElasticsearchExternalURL,
		AssociatedNamer:     esv1.ESNamer,
		AssociationName:     "beat-es",
		AssociatedShortName: "beat",
		AssociationLabels: func(associated types.NamespacedName) map[string]string {
			return map[string]string{
				BeatAssociationLabelName:      associated.Name,
				BeatAssociationLabelNamespace: associated.Namespace,
				BeatAssociationLabelType:      commonv1.ElasticsearchAssociationType,
			}
		},
		UserSecretSuffix:  "beat-user",
		CASecretLabelName: eslabel.ClusterNameLabelName,
		ESUserRole:        getBeatRoles,
	})
}

func getBeatRoles(assoc commonv1.Associated) (string, error) {
	beat, ok := assoc.(*beatv1beta1.Beat)
	if !ok {
		return "", pkgerrors.Errorf(
			"Beat expected, got %s/%s",
			assoc.GetObjectKind().GroupVersionKind().Group,
			assoc.GetObjectKind().GroupVersionKind().Kind,
		)
	}

	if strings.Contains(beat.Spec.Type, ",") {
		return "", fmt.Errorf("beat type %s should not contain a comma", beat.Spec.Type)
	}

	v, err := version.Parse(beat.Spec.Version)
	if err != nil {
		return "", err
	}

	// Roles for supported Beats are based on:
	// https://www.elastic.co/guide/en/beats/filebeat/current/feature-roles.html
	// Docs are the same for all Beats. For a specific version docs change "current" to major.minor, eg:
	// https://www.elastic.co/guide/en/beats/filebeat/7.1/feature-roles.html
	switch {
	case v.IsSameOrAfter(version.From(7, 7, 0)):
		return strings.Join([]string{
			"kibana_admin",
			"ingest_admin",
			"beats_admin",
			"remote_monitoring_user",
			esuser.BeatRoleName(esuser.V77, beat.Spec.Type),
		}, ","), nil
	case v.IsSameOrAfter(version.From(7, 5, 0)):
		return strings.Join([]string{
			"kibana_user",
			"ingest_admin",
			"beats_admin",
			"remote_monitoring_user",
			esuser.BeatRoleName(esuser.V75, beat.Spec.Type),
		}, ","), nil
	case v.IsSameOrAfter(version.From(7, 3, 0)):
		return strings.Join([]string{
			"kibana_user",
			"ingest_admin",
			"beats_admin",
			"remote_monitoring_user",
			esuser.BeatRoleName(esuser.V73, beat.Spec.Type),
		}, ","), nil
	default:
		return strings.Join([]string{
			"kibana_user",
			"ingest_admin",
			"beats_admin",
			"monitoring_user",
			esuser.BeatRoleName(esuser.V70, beat.Spec.Type),
		}, ","), nil
	}
}
