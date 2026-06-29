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
