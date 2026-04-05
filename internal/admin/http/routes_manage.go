package httpadmin

import "net/http"

// registerManagementRoutes returns admin configuration, users, themes, plugins,
// extensions, and audit routes.
func registerManagementRoutes(r *Router) []routeDef {
	routes := make([]routeDef, 0, 32)
	routes = append(routes, backupRoutes(r)...)
	routes = append(routes, operationsRoutes(r)...)
	routes = append(routes, settingsRoutes(r)...)
	routes = append(routes, userRoutes(r)...)
	routes = append(routes, themeRoutes(r)...)
	routes = append(routes, pluginRoutes(r)...)
	routes = append(routes, auditRoutes(r)...)
	return routes
}

func backupRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/backups"), handler: http.HandlerFunc(r.handleBackups), capability: "config.manage"},
		{pattern: r.routePath("/api/backups/create"), handler: http.HandlerFunc(r.handleCreateBackup), capability: "config.manage"},
		{pattern: r.routePath("/api/backups/restore"), handler: http.HandlerFunc(r.handleRestoreBackup), capability: "config.manage"},
		{pattern: r.routePath("/api/backups/download"), handler: http.HandlerFunc(r.handleDownloadBackup), capability: "config.manage"},
		{pattern: r.routePath("/api/backups/git"), handler: http.HandlerFunc(r.handleGitBackups), capability: "config.manage"},
		{pattern: r.routePath("/api/backups/git/create"), handler: http.HandlerFunc(r.handleCreateGitBackup), capability: "config.manage"},
	}
}

func operationsRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/operations"), handler: http.HandlerFunc(r.handleOperationsStatus), capability: "dashboard.read"},
		{pattern: r.routePath("/api/operations/logs"), handler: http.HandlerFunc(r.handleOperationsLogs), capability: "dashboard.read"},
		{pattern: r.routePath("/api/operations/validate"), handler: http.HandlerFunc(r.handleOperationsValidate), capability: "dashboard.read"},
		{pattern: r.routePath("/api/operations/cache/clear"), handler: http.HandlerFunc(r.handleOperationsClearCache), capability: "config.manage"},
		{pattern: r.routePath("/api/operations/rebuild"), handler: http.HandlerFunc(r.handleOperationsRebuild), capability: "config.manage"},
		{pattern: r.routePath("/api/update"), handler: http.HandlerFunc(r.handleUpdateStatus), capability: "dashboard.read"},
		{pattern: r.routePath("/api/update/apply"), handler: http.HandlerFunc(r.handleApplyUpdate), capability: "config.manage"},
	}
}

func settingsRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/extensions"), handler: http.HandlerFunc(r.handleExtensions), capability: "dashboard.read"},
		{pattern: r.routePath("/api/settings/sections"), handler: http.HandlerFunc(r.handleSettingsSections), capability: "dashboard.read"},
		{pattern: r.routePath("/api/settings/form"), handler: http.HandlerFunc(r.handleSettingsForm), capability: "config.manage"},
		{pattern: r.routePath("/api/settings/form/save"), handler: http.HandlerFunc(r.handleSaveSettingsForm), capability: "config.manage"},
		{pattern: r.routePath("/api/settings/custom-css"), handler: http.HandlerFunc(r.handleCustomCSSDocument), capability: "config.manage"},
		{pattern: r.routePath("/api/settings/custom-css/save"), handler: http.HandlerFunc(r.handleSaveCustomCSSDocument), capability: "config.manage"},
		{pattern: r.routePath("/api/custom-fields"), handler: http.HandlerFunc(r.handleCustomFieldsDocument), capability: "documents.read"},
		{pattern: r.routePath("/api/custom-fields/save"), handler: http.HandlerFunc(r.handleSaveCustomFieldsDocument), capability: "config.manage"},
		{pattern: r.routePath("/api/config"), handler: http.HandlerFunc(r.handleConfigDocument), capability: "config.manage"},
		{pattern: r.routePath("/api/config/save"), handler: http.HandlerFunc(r.handleSaveConfigDocument), capability: "config.manage"},
	}
}

func userRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/users"), handler: http.HandlerFunc(r.handleUsers), capability: "users.manage"},
		{pattern: r.routePath("/api/users/save"), handler: http.HandlerFunc(r.handleSaveUser), capability: "users.manage"},
		{pattern: r.routePath("/api/users/delete"), handler: http.HandlerFunc(r.handleDeleteUser), capability: "users.manage"},
	}
}

func themeRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/themes"), handler: http.HandlerFunc(r.handleThemes), capability: "themes.manage"},
		{pattern: r.routePath("/api/themes/install"), handler: http.HandlerFunc(r.handleInstallTheme), capability: "themes.manage"},
		{pattern: r.routePath("/api/themes/validate"), handler: http.HandlerFunc(r.handleValidateTheme), capability: "themes.manage"},
		{pattern: r.routePath("/api/themes/switch"), handler: http.HandlerFunc(r.handleThemeSwitch), capability: "themes.manage"},
	}
}

func pluginRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/plugins"), handler: http.HandlerFunc(r.handlePlugins), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/validate"), handler: http.HandlerFunc(r.handleValidatePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/install"), handler: http.HandlerFunc(r.handleInstallPlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/update"), handler: http.HandlerFunc(r.handleUpdatePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/rollback"), handler: http.HandlerFunc(r.handleRollbackPlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/enable"), handler: http.HandlerFunc(r.handleEnablePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/disable"), handler: http.HandlerFunc(r.handleDisablePlugin), capability: "plugins.manage"},
	}
}

func auditRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/audit"), handler: http.HandlerFunc(r.handleAudit), capability: "audit.read"},
	}
}
