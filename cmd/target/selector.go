package target

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	mcpv2 "github.com/openmcp-project/openmcp-operator/api/core/v2alpha1"
	pwv1alpha1 "github.com/openmcp-project/project-workspace-operator/api/core/v1alpha1"
)

// PROJECT

func projectSelectorPreview(project *pwv1alpha1.Project, _, _ int) string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "Name: %s\n", project.Name)

	// display name, if set
	if dName, ok := project.Labels["openmcp.cloud/display-name"]; ok {
		fmt.Fprintf(&sb, "Display Name: %s\n", dName)
	}

	// creator, if set
	if creator, ok := project.Labels["core.openmcp.cloud/created-by"]; ok {
		fmt.Fprintf(&sb, "Created By: %s\n", creator)
	}

	sb.WriteString("\nMembers:\n")
	for _, member := range project.Spec.Members {
		fmt.Fprintf(&sb, "- %s\n", projectMemberToString(member))
	}
	return sb.String()
}

func projectMemberToString(member pwv1alpha1.ProjectMember) string {
	res := projectMemberSubjectToString(member.Subject)
	sb := strings.Builder{}
	sb.WriteString(" (")
	appendedSomething := false
	for _, role := range member.Roles {
		if role == "" {
			continue
		}
		if appendedSomething {
			sb.WriteString(", ")
		}
		sb.WriteString(string(role))
		appendedSomething = true
	}
	if appendedSomething {
		sb.WriteString(")")
		res += sb.String()
	}
	return res
}

func projectMemberSubjectToString(subject pwv1alpha1.Subject) string {
	switch subject.Kind {
	case rbacv1.UserKind:
		return fmt.Sprintf("[User] %s", subject.Name)
	case rbacv1.GroupKind:
		return fmt.Sprintf("[Group] %s", subject.Name)
	case rbacv1.ServiceAccountKind:
		return fmt.Sprintf("[ServiceAccount] %s/%s", subject.Namespace, subject.Name)
	}
	return fmt.Sprintf("[Unknown] %s", subject.Name)
}

// WORKSPACE

func workspaceSelectorPreview(workspace *pwv1alpha1.Workspace, _, _ int) string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "Name: %s\n", workspace.Name)

	// display name, if set
	if dName, ok := workspace.Labels["openmcp.cloud/display-name"]; ok {
		fmt.Fprintf(&sb, "Display Name: %s\n", dName)
	}

	// creator, if set
	if creator, ok := workspace.Labels["core.openmcp.cloud/created-by"]; ok {
		fmt.Fprintf(&sb, "Created By: %s\n", creator)
	}

	sb.WriteString("\nMembers:\n")
	for _, member := range workspace.Spec.Members {
		fmt.Fprintf(&sb, "- %s\n", workspaceMemberToString(member))
	}
	return sb.String()
}

func workspaceMemberToString(member pwv1alpha1.WorkspaceMember) string {
	res := workspaceMemberSubjectToString(member.Subject)
	sb := strings.Builder{}
	sb.WriteString(" (")
	appendedSomething := false
	for _, role := range member.Roles {
		if role == "" {
			continue
		}
		if appendedSomething {
			sb.WriteString(", ")
		}
		sb.WriteString(string(role))
		appendedSomething = true
	}
	if appendedSomething {
		sb.WriteString(")")
		res += sb.String()
	}
	return res
}

func workspaceMemberSubjectToString(subject pwv1alpha1.Subject) string {
	switch subject.Kind {
	case rbacv1.UserKind:
		return fmt.Sprintf("[User] %s", subject.Name)
	case rbacv1.GroupKind:
		return fmt.Sprintf("[Group] %s", subject.Name)
	case rbacv1.ServiceAccountKind:
		return fmt.Sprintf("[ServiceAccount] %s/%s", subject.Namespace, subject.Name)
	}
	return fmt.Sprintf("[Unknown] %s", subject.Name)
}

// CONTROLPLANE

func cpSelectorPreview(mcp mcpv2.ControlPlane, _, _ int) string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "Name: %s\n", mcp.Name)

	fmt.Fprintf(&sb, "\nPhase: %s\n", mcp.Status.Phase)

	return sb.String()
}
