package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/NetSepio/erebrus/internal/store"
)

// ACLAction constants.
const (
	ActionConnect = "connect"
	ActionPublish = "publish"
	ActionManage  = "manage"
)

// ACLChecker evaluates service access policies.
type ACLChecker struct {
	St *store.Store
}

// AllowConnect returns whether subject may connect to the service.
func (a *ACLChecker) AllowConnect(ctx context.Context, svc *Service, subject string) (bool, error) {
	if svc == nil {
		return false, fmt.Errorf("service is nil")
	}
	switch svc.AuthMode {
	case "public":
		return true, nil
	case "vpn-peer":
		if subject == "" {
			return false, nil
		}
		if svc.OwnerPeerID != "" && subject == svc.OwnerPeerID {
			return true, nil
		}
		acls, err := a.St.ListServiceACLs(ctx, svc.ID)
		if err != nil {
			return false, err
		}
		for _, acl := range acls {
			if acl.Action != ActionConnect && acl.Action != "" {
				continue
			}
			if matchSubject(acl.Subject, subject) {
				return true, nil
			}
		}
		return svc.Visibility != "private" || svc.OwnerPeerID == subject, nil
	case "token":
		acls, err := a.St.ListServiceACLs(ctx, svc.ID)
		if err != nil {
			return false, err
		}
		for _, acl := range acls {
			if matchSubject(acl.Subject, subject) {
				return true, nil
			}
		}
		return false, nil
	default:
		return svc.Visibility == "public", nil
	}
}

func matchSubject(rule, subject string) bool {
	rule = strings.TrimSpace(rule)
	subject = strings.TrimSpace(subject)
	if rule == "public" {
		return true
	}
	if strings.HasPrefix(rule, "peer:") {
		return subject == strings.TrimPrefix(rule, "peer:")
	}
	if strings.HasPrefix(rule, "did:") {
		return subject == strings.TrimPrefix(rule, "did:")
	}
	if strings.HasPrefix(rule, "wallet:") {
		return strings.EqualFold(subject, strings.TrimPrefix(rule, "wallet:"))
	}
	return rule == subject
}

// Grant adds an ACL rule.
func (a *ACLChecker) Grant(ctx context.Context, serviceID, subject, action string) error {
	return a.St.InsertServiceACL(ctx, store.ServiceACL{
		ServiceID: serviceID,
		Subject:   subject,
		Action:    action,
	})
}
