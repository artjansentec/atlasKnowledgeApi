package service

import "github.com/atlas/knowledge-api/internal/domain"

func CanManageProject(user domain.User, project domain.Project) bool {
	return user.Role == domain.RoleAdmin || project.ResponsibleUserID == user.ID
}

func CanReadProject(user domain.User, project domain.Project, members []domain.ProjectMember) bool {
	if CanManageProject(user, project) {
		return true
	}
	for _, m := range members {
		if m.UserID == user.ID {
			return true
		}
	}
	return false
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
