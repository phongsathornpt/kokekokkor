package web

import (
	"context"
	"embed"
	_ "embed"
	"io"
)

//go:embed static/*
var Files embed.FS

//go:embed static/css/styles.css
var TailwindCSS string

// RenderDashboard renders the administrative dashboard using templ based on data.ActivePage.
func RenderDashboard(ctx context.Context, w io.Writer, data DashboardData) error {
	return Dashboard(data).Render(ctx, w)
}

// RenderOverview renders the overview page.
func RenderOverview(ctx context.Context, w io.Writer, data DashboardData) error {
	data.ActivePage = PageOverview
	return Overview(data).Render(ctx, w)
}

// RenderProviders renders the providers management page.
func RenderProviders(ctx context.Context, w io.Writer, data DashboardData) error {
	data.ActivePage = PageProviders
	return Providers(data).Render(ctx, w)
}

// RenderDefaults renders the protocol defaults page.
func RenderDefaults(ctx context.Context, w io.Writer, data DashboardData) error {
	data.ActivePage = PageDefaults
	return Defaults(data).Render(ctx, w)
}

// RenderRoutes renders the model routes page.
func RenderRoutes(ctx context.Context, w io.Writer, data DashboardData) error {
	data.ActivePage = PageRoutes
	return Routes(data).Render(ctx, w)
}

// RenderLogin renders the admin login page using templ.
func RenderLogin(ctx context.Context, w io.Writer, data LoginData) error {
	return Login(data).Render(ctx, w)
}
