//-----------------------------------------------------------------------------
// Copyright (c) 2020 Detlef Stern
//
// This file is part of zettelstore.
//
// Zettelstore is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// Zettelstore is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with Zettelstore. If not, see <http://www.gnu.org/licenses/>.
//-----------------------------------------------------------------------------

// Package main is the starting point for the zettel web command.
package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	_ "zettelstore.de/z/cmd"
	"zettelstore.de/z/config"
	"zettelstore.de/z/domain"
	"zettelstore.de/z/input"
	"zettelstore.de/z/parser"
	"zettelstore.de/z/store"
	"zettelstore.de/z/store/chainstore"
	"zettelstore.de/z/store/filestore"
	"zettelstore.de/z/store/gostore"
	"zettelstore.de/z/usecase"
	"zettelstore.de/z/web/adapter"
	"zettelstore.de/z/web/router"
)

// Version variables
var (
	buildVersion   string = ""
	releaseVersion string = ""
)

func setupRouting(s store.Store, readonly bool) *router.Router {
	te := adapter.NewTemplateEngine(s)
	p := parser.New()
	p.InitCache(s)

	ucGetMeta := usecase.NewGetMeta(s)
	ucGetZettel := usecase.NewGetZettel(s)
	listHTMLMetaHandler := adapter.MakeListHTMLMetaHandler('h', te, p, usecase.NewListMeta(s))
	getHTMLZettelHandler := adapter.MakeGetHTMLZettelHandler('h', te, p, ucGetZettel, ucGetMeta)
	getNewZettelHandler := adapter.MakeGetNewZettelHandler(te, ucGetZettel)
	postNewZettelHandler := adapter.MakePostNewZettelHandler(usecase.NewNewZettel(s))

	router := router.NewRouter()
	router.Handle("/", adapter.MakeGetRootHandler(s, listHTMLMetaHandler, getHTMLZettelHandler))
	if !readonly {
		router.AddListRoute('a', http.MethodGet, adapter.MakeGetLoginHandler(te))
		router.AddListRoute('a', http.MethodPost, adapter.MakePostLoginHandler(usecase.NewAuthenticate(s)))
	}
	router.AddZettelRoute('b', http.MethodGet, adapter.MakeGetBodyHandler('b', te, p, ucGetZettel, ucGetMeta))
	router.AddListRoute('c', http.MethodGet, adapter.MakeReloadHandler(usecase.NewReload(s)))
	router.AddZettelRoute('c', http.MethodGet, adapter.MakeGetContentHandler(ucGetZettel))
	if !readonly {
		router.AddZettelRoute('d', http.MethodGet, adapter.MakeGetDeleteZettelHandler(te, ucGetZettel))
		router.AddZettelRoute('d', http.MethodPost, adapter.MakePostDeleteZettelHandler(usecase.NewDeleteZettel(s)))
		router.AddZettelRoute('e', http.MethodGet, adapter.MakeEditGetZettelHandler(te, ucGetZettel))
		router.AddZettelRoute('e', http.MethodPost, adapter.MakeEditSetZettelHandler(usecase.NewUpdateZettel(s)))
	}
	router.AddListRoute('h', http.MethodGet, listHTMLMetaHandler)
	router.AddZettelRoute('h', http.MethodGet, getHTMLZettelHandler)
	router.AddZettelRoute('i', http.MethodGet, adapter.MakeGetInfoHandler(te, p, ucGetZettel, ucGetMeta))
	router.AddZettelRoute('m', http.MethodGet, adapter.MakeGetMetaHandler(p, ucGetMeta))
	if !readonly {
		router.AddListRoute('n', http.MethodGet, getNewZettelHandler)
		router.AddListRoute('n', http.MethodPost, postNewZettelHandler)
		router.AddZettelRoute('n', http.MethodGet, getNewZettelHandler)
		router.AddZettelRoute('n', http.MethodPost, postNewZettelHandler)
	}
	router.AddListRoute('r', http.MethodGet, adapter.MakeListRoleHandler(te, usecase.NewListRole(s)))
	if !readonly {
		router.AddZettelRoute('r', http.MethodGet, adapter.MakeGetRenameZettelHandler(te, ucGetMeta))
		router.AddZettelRoute('r', http.MethodPost, adapter.MakePostRenameZettelHandler(usecase.NewRenameZettel(s)))
	}
	router.AddListRoute('t', http.MethodGet, adapter.MakeListTagsHandler(te, usecase.NewListTags(s)))
	router.AddListRoute('s', http.MethodGet, adapter.MakeSearchHandler(te, p, usecase.NewSearch(s)))
	router.AddListRoute('z', http.MethodGet, adapter.MakeListMetaHandler('z', te, p, usecase.NewListMeta(s)))
	router.AddZettelRoute('z', http.MethodGet, adapter.MakeGetZettelHandler('z', te, p, ucGetZettel, ucGetMeta))
	return router
}

func setupConfig() (cfg *domain.Meta) {
	var configFile string
	var port uint64
	var dir string
	var readonly bool

	flag.StringVar(&configFile, "c", ".zscfg", "configuration file")
	flag.Uint64Var(&port, "p", 23123, "port number")
	flag.StringVar(&dir, "d", "./zettel", "zettel directory")
	flag.BoolVar(&readonly, "r", false, "system read-only mode")
	flag.Parse()

	if content, err := ioutil.ReadFile(configFile); err != nil {
		cfg = domain.NewMeta("")
	} else {
		cfg = domain.NewMetaFromInput("", input.NewInput(string(content)))
	}
	flag.Visit(func(flg *flag.Flag) {
		switch flg.Name {
		case "p":
			cfg.Set("listen-addr", ":"+flg.Value.String())
		case "d":
			cfg.Set("store-dir-1", flg.Value.String())
		case "r":
			cfg.Set("readonly", flg.Value.String())
		}
	})

	if _, ok := cfg.Get("listen-addr"); !ok {
		cfg.Set("listen-addr", ":23123")
	}
	if _, ok := cfg.Get("store-dir-1"); !ok {
		cfg.Set("store-dir-1", "./zettel")
	}
	if _, ok := cfg.Get("readonly"); !ok {
		cfg.Set("readonly", "0")
	}
	cfg.Set("release-version", releaseVersion)
	cfg.Set("build-version", buildVersion)
	return cfg
}

func main() {
	cfg := setupConfig()
	config.SetupStartup(cfg)

	dir, _ := cfg.Get("store-dir-1")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Unable to create zettel directory %q: %v", dir, err)
	}
	fs, err := filestore.NewStore(dir)
	if err != nil {
		log.Fatalf("Unable to create filestore for %q: %v", dir, err)
	}
	cs := chainstore.NewStore(fs, gostore.NewStore())
	if err = cs.Start(context.Background()); err != nil {
		log.Fatalf("Unable to start zettel store: %v", err)
	}
	config.SetupConfiguration(cs)

	readonly := cfg.GetBool("readonly")
	router := setupRouting(cs, readonly)

	v := config.Config.GetVersion()
	log.Printf("Release %v, Build %v", v.Release, v.Build)
	listenAddr, _ := cfg.Get("listen-addr")
	log.Printf("Listening on %v", listenAddr)
	log.Printf("Zettel location %q", cs.Location())
	if readonly {
		log.Println("Read-only node")
	}
	log.Fatal(http.ListenAndServe(listenAddr, router))
}
