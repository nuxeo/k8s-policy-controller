package reviewer

import (
	"encoding/json"

	"github.com/nuxeo/k8s-policy-controller/pkg/plugins/gcpauthpolicy/k8s"
	"github.com/pkg/errors"

	gcpauth_api "github.com/nuxeo/k8s-policy-controller/apis/gcpauthpolicyprofile/v1alpha1"
	core_api "k8s.io/api/core/v1"
	meta_api "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"

	spi "github.com/nuxeo/k8s-policy-controller/pkg/plugins/spi/reviewer"
)

type (
	RequestedServiceAccountStage struct {
		k8s.Interface
		*spi.GivenStage
		core_api.ServiceAccount
		gcpauth_api.Profile
	}
	RequestedKindStage struct {
		*RequestedServiceAccountStage
	}
	RequestedProfileStage struct {
		*RequestedServiceAccountStage
		*core_api.Namespace
	}
)

func Given() *RequestedServiceAccountStage {
	return &RequestedServiceAccountStage{}
}

func (s *RequestedServiceAccountStage) RequestedObject(o *spi.GivenStage) *RequestedServiceAccountStage {
	s.GivenStage = o
	s.Interface = k8s.Interface{Interface: o.Interface}
	return s
}

func (s *RequestedServiceAccountStage) The() *RequestedServiceAccountStage {
	return s
}

func (s *RequestedServiceAccountStage) And() *RequestedServiceAccountStage {
	return s
}

func (r *RequestedServiceAccountStage) RequestedKind() *RequestedKindStage {
	return &RequestedKindStage{r}
}

func (s *RequestedKindStage) Or() *RequestedKindStage {
	return s
}

func (s *RequestedKindStage) IsAServiceAccount() *RequestedKindStage {
	err := json.Unmarshal(s.AdmissionRequest.Object.Raw, &s.ServiceAccount)
	if err != nil {
		s.Allow(nil)
		return s
	}
	s.Logger = s.Logger.WithValues("name", s.ServiceAccount.ObjectMeta.Name)

	return s
}

func (s *RequestedKindStage) End() *RequestedServiceAccountStage {
	return s.RequestedServiceAccountStage
}

func (s *RequestedServiceAccountStage) RequestedProfile() *RequestedProfileStage {
	return &RequestedProfileStage{s, nil}
}

func (s *RequestedProfileStage) Applies() *RequestedProfileStage {
	if !s.CanContinue() {
		return s
	}

	s.Namespace, s.Error = s.Interface.GetNamespace(s.AdmissionRequest.Namespace)
	if s.Error != nil {
		s.Allow(nil)
		return s
	}

	profile, err := s.Interface.ResolveProfile(&s.Namespace.ObjectMeta, &s.ServiceAccount.ObjectMeta)
	if err != nil {
		s.Allow(err)
		return s
	}
	s.Profile = *profile

	if s.Profile.Spec.Selector != nil {
		selector, err := meta_api.LabelSelectorAsSelector(s.Profile.Spec.Selector)
		if err != nil {
			s.Error = err
			s.Allow(nil)
			return s
		}
		if !selector.Matches(labels.Set(s.ServiceAccount.Labels)) {
			s.Allow(nil)
			return s
		}
	}

	s.Logger = s.Logger.WithValues("profile", s.Profile.ObjectMeta.Name)

	return s
}

func (s *RequestedProfileStage) SecretIsAvailable() *RequestedProfileStage {
	if !s.CanContinue() {
		return s
	}
	err := s.Interface.EnsureNamespaceImagePullSecret(&s.Profile, s.ServiceAccount.ObjectMeta.Namespace)
	if err != nil {
		s.Fail(errors.Wrap(err, "Cannot ensure we have an image pull secret available"))
	}
	return s
}

func (s *RequestedProfileStage) And() *RequestedProfileStage {
	return s
}

func (s *RequestedProfileStage) The() *RequestedProfileStage {
	return s
}

func (s *RequestedProfileStage) End() *RequestedServiceAccountStage {
	return s.RequestedServiceAccountStage
}

func (s *RequestedServiceAccountStage) End() *spi.WhenStage {
	return &spi.WhenStage{
		GivenStage: s.GivenStage,
		Patcher: &serviceaccountPatcher{
			ServiceAccount: &s.ServiceAccount,
			Profile:        &s.Profile,
		}}
}
