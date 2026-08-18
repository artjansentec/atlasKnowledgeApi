package service

import "github.com/atlas/knowledge-api/internal/domain"

func CanManageProject(user domain.User, project domain.Project) bool {
	return user.Role == domain.RoleAdmin || project.ResponsibleUserID == user.ID
}

func CanReadProject(user domain.User, _ domain.Project, _ []domain.ProjectMember) bool {
	return user.ID != ""
}

func IsAdmin(user domain.User) bool {
	return user.Role == domain.RoleAdmin
}

// CanManageDevSections define quem edita a aba Desenvolvimento: admin ou
// dev-responsável do projeto.
func CanManageDevSections(user domain.User, devResponsibleIDs []string) bool {
	if user.Role == domain.RoleAdmin {
		return true
	}
	for _, id := range devResponsibleIDs {
		if id == user.ID {
			return true
		}
	}
	return false
}

// CanSeeDevSections define quem visualiza a aba Desenvolvimento: admin e
// desenvolvedores (consultor não recebe dev-sections).
func CanSeeDevSections(user domain.User) bool {
	return user.Role == domain.RoleAdmin || user.Role == domain.RoleDeveloper
}
