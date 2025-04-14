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
	"zettelstore.de/z/internal/web/adapter/api"
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
	webLog := kern.GetLogger(kernel.WebService)

	var getUser getUserImpl
	logAuth := kern.GetLogger(kernel.AuthService)
	logUc := kern.GetLogger(kernel.CoreService).WithUser(&getUser)
	ucGetUser := usecase.NewGetUser(authManager, boxManager)
	ucAuthenticate := usecase.NewAuthenticate(logAuth, authManager, &ucGetUser)
	ucIsAuth := usecase.NewIsAuthenticated(logUc, &getUser, authManager)
	ucCreateZettel := usecase.NewCreateZettel(logUc, rtConfig, protectedBoxManager)
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
	ucDelete := usecase.NewDeleteZettel(logUc, protectedBoxManager)
	ucUpdate := usecase.NewUpdateZettel(logUc, protectedBoxManager)
	ucRefresh := usecase.NewRefresh(logUc, protectedBoxManager)
	ucReIndex := usecase.NewReIndex(logUc, protectedBoxManager)
	ucVersion := usecase.NewVersion(kernel.Main.GetConfig(kernel.CoreService, kernel.CoreVersion).(string))

	a := api.New(
		webLog.Clone().Str("adapter", "api").Child(),
		webSrv, authManager, authManager, rtConfig, authPolicy)
	wui := webui.New(
		webLog.Clone().Str("adapter", "wui").Child(),
		webSrv, authManager, rtConfig, authManager, boxManager, authPolicy, &ucEvaluate)

	webSrv.Handle("/", wui.MakeGetRootHandler(protectedBoxManager))
	if assetDir := kern.GetConfig(kernel.WebService, kernel.WebAssetDir).(string); assetDir != "" {
		const assetPrefix = "/assets/"
		webSrv.Handle(assetPrefix, http.StripPrefix(assetPrefix, http.FileServer(http.Dir(assetDir))))
		webSrv.Handle("/favicon.ico", wui.MakeFaviconHandler(assetDir))
	}

	// Web user interface
	if !authManager.IsReadonly() {
		webSrv.AddListRoute('c', server.MethodGet, wui.MakeGetZettelFromListHandler(&ucQuery, &ucEvaluate, ucListRoles, ucListSyntax))
		webSrv.AddListRoute('c', server.MethodPost, wui.MakePostCreateZettelHandler(&ucCreateZettel))
		webSrv.AddZettelRoute('c', server.MethodGet, wui.MakeGetCreateZettelHandler(
			ucGetZettel, &ucCreateZettel, ucListRoles, ucListSyntax))
		webSrv.AddZettelRoute('c', server.MethodPost, wui.MakePostCreateZettelHandler(&ucCreateZettel))
		webSrv.AddZettelRoute('d', server.MethodGet, wui.MakeGetDeleteZettelHandler(ucGetZettel, ucGetAllZettel))
		webSrv.AddZettelRoute('d', server.MethodPost, wui.MakePostDeleteZettelHandler(&ucDelete))
		webSrv.AddZettelRoute('e', server.MethodGet, wui.MakeEditGetZettelHandler(ucGetZettel, ucListRoles, ucListSyntax))
		webSrv.AddZettelRoute('e', server.MethodPost, wui.MakeEditSetZettelHandler(&ucUpdate))
	}
	webSrv.AddListRoute('g', server.MethodGet, wui.MakeGetGoActionHandler(&ucRefresh))
	webSrv.AddListRoute('h', server.MethodGet, wui.MakeListHTMLMetaHandler(&ucQuery, &ucTagZettel, &ucRoleZettel, &ucReIndex))
	webSrv.AddZettelRoute('h', server.MethodGet, wui.MakeGetHTMLZettelHandler(&ucEvaluate, ucGetZettel))
	webSrv.AddListRoute('i', server.MethodGet, wui.MakeGetLoginOutHandler())
	webSrv.AddListRoute('i', server.MethodPost, wui.MakePostLoginHandler(&ucAuthenticate))
	webSrv.AddZettelRoute('i', server.MethodGet, wui.MakeGetInfoHandler(
		ucParseZettel, ucGetReferences, &ucEvaluate, ucGetZettel, ucGetAllZettel, &ucQuery))

	// API
	webSrv.AddListRoute('a', server.MethodPost, a.MakePostLoginHandler(&ucAuthenticate))
	webSrv.AddListRoute('a', server.MethodPut, a.MakeRenewAuthHandler())
	webSrv.AddListRoute('x', server.MethodGet, a.MakeGetDataHandler(ucVersion))
	webSrv.AddListRoute('x', server.MethodPost, a.MakePostCommandHandler(&ucIsAuth, &ucRefresh))
	webSrv.AddListRoute('z', server.MethodGet, a.MakeQueryHandler(&ucQuery, &ucTagZettel, &ucRoleZettel, &ucReIndex))
	webSrv.AddZettelRoute('z', server.MethodGet, a.MakeGetZettelHandler(ucGetZettel, ucParseZettel, ucEvaluate))
	if !authManager.IsReadonly() {
		webSrv.AddListRoute('z', server.MethodPost, a.MakePostCreateZettelHandler(&ucCreateZettel))
		webSrv.AddZettelRoute('z', server.MethodPut, a.MakeUpdateZettelHandler(&ucUpdate))
		webSrv.AddZettelRoute('z', server.MethodDelete, a.MakeDeleteZettelHandler(&ucDelete))
	}

	if authManager.WithAuth() {
		webSrv.SetUserRetriever(usecase.NewGetUserByZid(boxManager))
	}
}

type getUserImpl struct{}

func (*getUserImpl) GetUser(ctx context.Context) *meta.Meta { return server.GetUser(ctx) }
