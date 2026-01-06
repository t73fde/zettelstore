//-----------------------------------------------------------------------------
// Copyright (c) 2020-present Detlef Stern
//
// This file is part of Zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2020-present Detlef Stern
//-----------------------------------------------------------------------------

package cmd

import (
	"context"
	"flag"
	"net/http"

	"t73f.de/r/zsc/domain/meta"

	"zettelstore.de/z/internal/auth"
	"zettelstore.de/z/internal/box"
	"zettelstore.de/z/internal/config"
	"zettelstore.de/z/internal/kernel"
	"zettelstore.de/z/internal/usecase"
	"zettelstore.de/z/internal/web/adapter/webapi"
	"zettelstore.de/z/internal/web/adapter/webui"
	"zettelstore.de/z/internal/web/server"
)

// ---------- Subcommand: run ------------------------------------------------

func flgRun(fs *flag.FlagSet) {
	fs.String("c", "", "configuration file")
	fs.Uint("a", 0, "port number kernel service (0=disable)")
	fs.Uint("p", 23123, "port number web service")
	fs.String("d", "", "zettel directory")
	fs.Bool("r", false, "system-wide read-only mode")
	fs.Bool("v", false, "verbose mode")
	fs.Bool("debug", false, "debug mode")
}

func runFunc(*flag.FlagSet) (int, error) {
	var exitCode int
	err := kernel.Main.StartService(kernel.WebService)
	if err != nil {
		exitCode = 1
	}
	kernel.Main.WaitForShutdown()
	return exitCode, err
}

func setupRouting(webSrv server.Server, boxManager box.Manager, authManager auth.Manager, rtConfig config.Config) {
	protectedBoxManager, authPolicy := authManager.BoxWithPolicy(boxManager, rtConfig)
	kern := kernel.Main
	webLogger := kern.GetLogger(kernel.WebService)

	var getUser getUserImpl
	authLogger := kern.GetLogger(kernel.AuthService)
	ucLogger := kern.GetLogger(kernel.CoreService)
	ucGetUser := usecase.NewGetUser(authManager, boxManager)
	ucAuthenticate := usecase.NewAuthenticate(authLogger, authManager, &ucGetUser)
	ucIsAuth := usecase.NewIsAuthenticated(ucLogger, &getUser, authManager)
	ucCreateZettel := usecase.NewCreateZettel(ucLogger, rtConfig, protectedBoxManager)
	ucGetAllZettel := usecase.NewGetAllZettel(protectedBoxManager)
	ucGetZettel := usecase.NewGetZettel(protectedBoxManager)
	ucParseZettel := usecase.NewParseZettel(rtConfig, ucGetZettel)
	ucGetReferences := usecase.NewGetReferences()
	ucQuery := usecase.NewQuery(protectedBoxManager)
	ucEvaluate := usecase.NewEvaluate(rtConfig, &ucGetZettel, &ucQuery)
	ucQuery.SetEvaluate(&ucEvaluate)
	ucTagZettel := usecase.NewTagZettel(protectedBoxManager, &ucQuery)
	ucRoleZettel := usecase.NewRoleZettel(protectedBoxManager, &ucQuery)
	ucListSyntax := usecase.NewListSyntax(protectedBoxManager)
	ucListRoles := usecase.NewListRoles(protectedBoxManager)
	ucDelete := usecase.NewDeleteZettel(ucLogger, protectedBoxManager)
	ucUpdate := usecase.NewUpdateZettel(ucLogger, protectedBoxManager)
	ucRefresh := usecase.NewRefresh(ucLogger, protectedBoxManager)
	ucReIndex := usecase.NewReIndex(ucLogger, protectedBoxManager)
	ucVersion := usecase.NewVersion(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVersion).(string))

	a := webapi.New(
		webLogger.With("system", "WEBAPI"),
		webSrv, authManager, authManager, rtConfig, authPolicy)
	wui := webui.New(
		webLogger.With("system", "WEBUI"),
		webSrv, authManager, rtConfig, authManager, boxManager, authPolicy, &ucEvaluate)

	webSrv.Handle("/", wui.MakeGetRootHandler(protectedBoxManager))
	if assetDir := kern.GetConfig(kernel.WebService, kernel.WebAssetDir).(string); assetDir != "" {
		const assetPrefix = "/assets/"
		webSrv.Handle(assetPrefix, http.StripPrefix(assetPrefix, http.FileServer(http.Dir(assetDir))))
		webSrv.Handle("/favicon.ico", wui.MakeFaviconHandler(assetDir))
	}

	const isAPI = true

	// Web user interface
	if !authManager.IsReadonly() {
		webSrv.AddListRoute(!isAPI, 'c', server.MethodGet, wui.MakeGetZettelFromListHandler(&ucQuery, &ucEvaluate, ucListRoles, ucListSyntax))
		webSrv.AddListRoute(!isAPI, 'c', server.MethodPost, wui.MakePostCreateZettelHandler(&ucCreateZettel))
		webSrv.AddZettelRoute(!isAPI, 'c', server.MethodGet, wui.MakeGetCreateZettelHandler(
			ucGetZettel, &ucCreateZettel, ucListRoles, ucListSyntax))
		webSrv.AddZettelRoute(!isAPI, 'c', server.MethodPost, wui.MakePostCreateZettelHandler(&ucCreateZettel))
		webSrv.AddZettelRoute(!isAPI, 'd', server.MethodGet, wui.MakeGetDeleteZettelHandler(ucGetZettel, ucGetAllZettel))
		webSrv.AddZettelRoute(!isAPI, 'd', server.MethodPost, wui.MakePostDeleteZettelHandler(&ucDelete))
		webSrv.AddZettelRoute(!isAPI, 'e', server.MethodGet, wui.MakeEditGetZettelHandler(ucGetZettel, ucListRoles, ucListSyntax))
		webSrv.AddZettelRoute(!isAPI, 'e', server.MethodPost, wui.MakeEditSetZettelHandler(&ucUpdate))
	}
	webSrv.AddListRoute(!isAPI, 'g', server.MethodGet, wui.MakeGetGoActionHandler(&ucRefresh))
	webSrv.AddListRoute(!isAPI, 'h', server.MethodGet, wui.MakeListHTMLMetaHandler(&ucQuery, &ucTagZettel, &ucRoleZettel, &ucReIndex))
	webSrv.AddZettelRoute(!isAPI, 'h', server.MethodGet, wui.MakeGetHTMLZettelHandler(&ucEvaluate, ucGetZettel))
	webSrv.AddListRoute(!isAPI, 'i', server.MethodGet, wui.MakeGetLoginOutHandler())
	webSrv.AddListRoute(!isAPI, 'i', server.MethodPost, wui.MakePostLoginHandler(&ucAuthenticate))
	webSrv.AddZettelRoute(!isAPI, 'i', server.MethodGet, wui.MakeGetInfoHandler(
		ucParseZettel, ucGetReferences, &ucEvaluate, ucGetZettel, ucGetAllZettel, &ucQuery))

	// API
	webSrv.AddListRoute(isAPI, 'a', server.MethodPost, a.MakePostLoginHandler(&ucAuthenticate))
	webSrv.AddListRoute(isAPI, 'a', server.MethodPut, a.MakeRenewAuthHandler())
	webSrv.AddZettelRoute(isAPI, 'r', server.MethodGet, a.MakeGetReferencesHandler(ucParseZettel, ucGetReferences))
	webSrv.AddListRoute(isAPI, 'x', server.MethodGet, a.MakeGetDataHandler(ucVersion))
	webSrv.AddListRoute(isAPI, 'x', server.MethodPost, a.MakePostCommandHandler(&ucIsAuth, &ucRefresh))
	webSrv.AddListRoute(isAPI, 'z', server.MethodGet, a.MakeQueryHandler(&ucQuery, &ucTagZettel, &ucRoleZettel, &ucReIndex))
	webSrv.AddZettelRoute(isAPI, 'z', server.MethodGet, a.MakeGetZettelHandler(ucGetZettel, ucParseZettel, ucEvaluate))
	if !authManager.IsReadonly() {
		webSrv.AddListRoute(isAPI, 'z', server.MethodPost, a.MakePostCreateZettelHandler(&ucCreateZettel))
		webSrv.AddZettelRoute(isAPI, 'z', server.MethodPut, a.MakeUpdateZettelHandler(&ucUpdate))
		webSrv.AddZettelRoute(isAPI, 'z', server.MethodDelete, a.MakeDeleteZettelHandler(&ucDelete))
	}

	if authManager.WithAuth() {
		webSrv.SetUserRetriever(usecase.NewGetUserByZid(boxManager))
	}
}

type getUserImpl struct{}

func (*getUserImpl) GetCurrentUser(ctx context.Context) *meta.Meta { return auth.GetCurrentUser(ctx) }
