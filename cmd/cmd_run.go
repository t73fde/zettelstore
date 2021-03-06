//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is licensed under the latest version of the EUPL (European Union
// Public License). Please see file LICENSE.txt for your rights and obligations
// under this license.
//-----------------------------------------------------------------------------

package cmd

import (
	"flag"
	"log"
	"net/http"

	"zettelstore.de/z/auth/policy"
	"zettelstore.de/z/config/runtime"
	"zettelstore.de/z/config/startup"
	"zettelstore.de/z/place"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/adapter/api"
	"zettelstore.de/z/web/adapter/webui"
	"zettelstore.de/z/web/router"
	"zettelstore.de/z/web/server"
	"zettelstore.de/z/web/session"
)

// ---------- Subcommand: run ------------------------------------------------

func flgRun(fs *flag.FlagSet) {
	fs.String("c", defConfigfile, "configuration file")
	fs.Uint("p", 23123, "port number")
	fs.String("d", "", "zettel directory")
	fs.Bool("r", false, "system-wide read-only mode")
	fs.Bool("v", false, "verbose mode")
	fs.Bool("debug", false, "debug mode")
}

func enableDebug(fs *flag.FlagSet, srv *server.Server) {
	if dbg := fs.Lookup("debug"); dbg != nil && dbg.Value.String() == "true" {
		srv.SetDebug()
	}
}

func runFunc(fs *flag.FlagSet) (int, error) {
	listenAddr := startup.ListenAddress()
	readonlyMode := startup.IsReadOnlyMode()
	logBeforeRun(listenAddr, readonlyMode)
	handler := setupRouting(startup.PlaceManager(), readonlyMode)
	srv := server.New(listenAddr, handler)
	enableDebug(fs, srv)
	if err := srv.Run(); err != nil {
		return 1, err
	}
	return 0, nil
}

func logBeforeRun(listenAddr string, readonlyMode bool) {
	v := startup.GetVersion()
	log.Printf("%v %v (%v@%v/%v)", v.Prog, v.Build, v.GoVersion, v.Os, v.Arch)
	log.Println("Licensed under the latest version of the EUPL (European Union Public License)")
	log.Printf("Listening on %v", listenAddr)
	log.Printf("Zettel location [%v]", startup.PlaceManager().Location())
	if readonlyMode {
		log.Println("Read-only mode")
	}
}

func setupRouting(up place.Place, readonlyMode bool) http.Handler {
	pp, pol := policy.PlaceWithPolicy(
		up, startup.IsSimple(), startup.WithAuth, readonlyMode, runtime.GetExpertMode,
		startup.IsOwner, runtime.GetVisibility)
	te := webui.NewTemplateEngine(up, pol)

	ucAuthenticate := usecase.NewAuthenticate(up)
	ucGetMeta := usecase.NewGetMeta(pp)
	ucGetZettel := usecase.NewGetZettel(pp)
	ucParseZettel := usecase.NewParseZettel(ucGetZettel)
	ucListMeta := usecase.NewListMeta(pp)
	ucListRoles := usecase.NewListRole(pp)
	ucListTags := usecase.NewListTags(pp)
	listHTMLMetaHandler := webui.MakeListHTMLMetaHandler(te, ucListMeta)
	getHTMLZettelHandler := webui.MakeGetHTMLZettelHandler(te, ucParseZettel, ucGetMeta)

	router := router.NewRouter()
	router.Handle("/", webui.MakeGetRootHandler(
		pp, listHTMLMetaHandler, getHTMLZettelHandler))
	router.AddListRoute('a', http.MethodGet, webui.MakeGetLoginHandler(te))
	router.AddListRoute('a', http.MethodPost, adapter.MakePostLoginHandler(
		api.MakePostLoginHandlerAPI(ucAuthenticate),
		webui.MakePostLoginHandlerHTML(te, ucAuthenticate)))
	router.AddListRoute('a', http.MethodPut, api.MakeRenewAuthHandler())
	router.AddZettelRoute('a', http.MethodGet, webui.MakeGetLogoutHandler())
	router.AddListRoute('c', http.MethodGet, adapter.MakeReloadHandler(
		usecase.NewReload(pp), api.ReloadHandlerAPI, webui.ReloadHandlerHTML))
	if !readonlyMode {
		router.AddZettelRoute('c', http.MethodGet, webui.MakeGetCopyZettelHandler(
			te, ucGetZettel, usecase.NewCopyZettel()))
		router.AddZettelRoute('c', http.MethodPost, webui.MakePostCreateZettelHandler(
			usecase.NewCreateZettel(pp)))
		router.AddZettelRoute('d', http.MethodGet, webui.MakeGetDeleteZettelHandler(
			te, ucGetZettel))
		router.AddZettelRoute('d', http.MethodPost, webui.MakePostDeleteZettelHandler(
			usecase.NewDeleteZettel(pp)))
		router.AddZettelRoute('e', http.MethodGet, webui.MakeEditGetZettelHandler(
			te, ucGetZettel))
		router.AddZettelRoute('e', http.MethodPost, webui.MakeEditSetZettelHandler(
			usecase.NewUpdateZettel(pp)))
		router.AddZettelRoute('f', http.MethodGet, webui.MakeGetFolgeZettelHandler(
			te, ucGetZettel, usecase.NewFolgeZettel()))
		router.AddZettelRoute('f', http.MethodPost, webui.MakePostCreateZettelHandler(
			usecase.NewCreateZettel(pp)))
	}
	router.AddListRoute('h', http.MethodGet, listHTMLMetaHandler)
	router.AddZettelRoute('h', http.MethodGet, getHTMLZettelHandler)
	router.AddZettelRoute('i', http.MethodGet, webui.MakeGetInfoHandler(
		te, ucParseZettel, ucGetMeta))
	router.AddZettelRoute('k', http.MethodGet, webui.MakeWebUIListsHandler(
		te, ucListMeta, ucListRoles, ucListTags))
	router.AddZettelRoute('l', http.MethodGet, api.MakeGetLinksHandler(ucParseZettel))
	if !readonlyMode {
		router.AddZettelRoute('n', http.MethodGet, webui.MakeGetNewZettelHandler(
			te, ucGetZettel, usecase.NewNewZettel()))
		router.AddZettelRoute('n', http.MethodPost, webui.MakePostCreateZettelHandler(
			usecase.NewCreateZettel(pp)))
	}
	router.AddListRoute('r', http.MethodGet, api.MakeListRoleHandler(ucListRoles))
	if !readonlyMode {
		router.AddZettelRoute('r', http.MethodGet, webui.MakeGetRenameZettelHandler(
			te, ucGetMeta))
		router.AddZettelRoute('r', http.MethodPost, webui.MakePostRenameZettelHandler(
			usecase.NewRenameZettel(pp)))
	}
	router.AddListRoute('t', http.MethodGet, api.MakeListTagsHandler(ucListTags))
	router.AddListRoute('s', http.MethodGet, webui.MakeSearchHandler(
		te, usecase.NewSearch(pp), ucGetMeta, ucGetZettel))
	router.AddListRoute('z', http.MethodGet, api.MakeListMetaHandler(
		usecase.NewListMeta(pp), ucGetMeta, ucParseZettel))
	router.AddZettelRoute('z', http.MethodGet, api.MakeGetZettelHandler(
		ucParseZettel, ucGetMeta))
	return session.NewHandler(router, usecase.NewGetUserByZid(up))
}
