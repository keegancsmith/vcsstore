package client

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/go-vcs/vcs"
	"github.com/sqs/mux"
)

const (
	// Route names
	RouteRepo          = "repo"
	RouteRepoBranch    = "repo.branch"
	RouteRepoCommit    = "repo.commit"
	RouteRepoRevision  = "repo.rev"
	RouteRepoTag       = "repo.tag"
	RouteRepoTreeEntry = "repo.tree-entry"
	RouteRoot          = "root"
)

type Router mux.Router

// NewRouter creates a new router that matches and generates URLs that the HTTP
// handler recognizes.
func NewRouter() *Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	r.Path("/").Methods("GET").Name(RouteRoot)

	unescapeRepoVars := func(req *http.Request, match *mux.RouteMatch, r *mux.Route) {
		esc := strings.Replace(match.Vars["CloneURLEscaped"], "$", "%2F", -1)
		match.Vars["CloneURL"], _ = url.QueryUnescape(esc)
		delete(match.Vars, "CloneURLEscaped")
	}
	escapeRepoVars := func(vars map[string]string) map[string]string {
		esc := url.QueryEscape(vars["CloneURL"])
		vars["CloneURLEscaped"] = strings.Replace(esc, "%2F", "$", -1)
		delete(vars, "CloneURL")
		return vars
	}

	repoPath := "/repos/{VCS}/{CloneURLEscaped:[^/]+}"
	r.Path(repoPath).Methods("GET").PostMatchFunc(unescapeRepoVars).BuildVarsFunc(escapeRepoVars).Name(RouteRepo)
	repo := r.PathPrefix(repoPath).PostMatchFunc(unescapeRepoVars).BuildVarsFunc(escapeRepoVars).Subrouter()
	repo.Path("/branches/{Branch}").Methods("GET").Name(RouteRepoBranch)
	repo.Path("/revs/{RevSpec}").Methods("GET").Name(RouteRepoRevision)
	repo.Path("/tags/{Tag}").Methods("GET").Name(RouteRepoTag)
	commitPath := "/commits/{CommitID}"
	repo.Path(commitPath).Methods("GET").Name(RouteRepoCommit)
	commit := repo.PathPrefix(commitPath).Subrouter()

	// cleanTreeVars modifies the Path route var to be a clean filepath. If it
	// is empty, it is changed to ".".
	cleanTreeVars := func(req *http.Request, match *mux.RouteMatch, r *mux.Route) {
		path := filepath.Clean(strings.TrimPrefix(match.Vars["Path"], "/"))
		if path == "" || path == "." {
			match.Vars["Path"] = "."
		} else {
			match.Vars["Path"] = path
		}
	}
	// prepareTreeVars prepares the Path route var to generate a clean URL.
	prepareTreeVars := func(vars map[string]string) map[string]string {
		if path := vars["Path"]; path == "." {
			vars["Path"] = ""
		} else {
			vars["Path"] = "/" + filepath.Clean(path)
		}
		return vars
	}
	commit.Path("/tree{Path:(?:/.*)*}").Methods("GET").PostMatchFunc(cleanTreeVars).BuildVarsFunc(prepareTreeVars).Name(RouteRepoTreeEntry)

	return (*Router)(r)
}

func (r *Router) URLToRepoCommit(vcsType string, cloneURL *url.URL, commitID vcs.CommitID) *url.URL {
	return r.URLTo(RouteRepoCommit, "VCS", vcsType, "CloneURL", cloneURL.String(), "CommitID", string(commitID))
}

func (r *Router) URLTo(route string, vars ...string) *url.URL {
	url, err := (*mux.Router)(r).Get(route).URL(vars...)
	if err != nil {
		panic(err.Error())
	}
	return url
}
