package apollostats

import (
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Apollo-Community/ApolloStats/src/assetstatic"
	"github.com/Apollo-Community/ApolloStats/src/assettemplates"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
)

type Instance struct {
	Verbose bool
	DB      *DB

	addr   string
	router *gin.Engine
	cache  *Cache
}

func (i *Instance) Init() error {
	i.cache = NewCache(i)
	gin.SetMode(gin.ReleaseMode)
	i.router = gin.New()
	i.router.Use(gin.Recovery())
	i.router.Use(i.logger())

	// Custom functions for the templates
	funcmap := template.FuncMap{
		"pretty_time": func(t time.Time) string {
			return t.UTC().Format("2006-01-02 15:04 MST")
		},
		"pretty_duration": func(d time.Duration) string {
			m := d.Minutes()
			if m >= 1 {
				return fmt.Sprintf("%.1fmin", m)
			}
			return fmt.Sprintf("%.1fsec", d.Seconds())
		},
		"year": func() int {
			return time.Now().Year()
		},
		"commas": func(i int64) string {
			return humanize.Comma(i)
		},
		"round": func(f float64) string {
			return fmt.Sprintf("%.1f", f)
		},
		"default_job": func(s string) string {
			if len(strings.TrimSpace(s)) < 1 {
				return "Unknown"
			}
			return s
		},
	}

	// Load templates
	tmpl := template.New("AllTemplates").Funcs(funcmap)
	tmplfiles, e := assettemplates.AssetDir("templates/")
	if e != nil {
		return e
	}
	for p, b := range tmplfiles {
		name := filepath.Base(p)
		_, e = tmpl.New(name).Parse(string(b))
		if e != nil {
			return e
		}
	}
	i.router.SetHTMLTemplate(tmpl)

	// And static files
	staticfiles, e := assetstatic.AssetDir("static/")
	if e != nil {
		return e
	}
	for p, _ := range staticfiles {
		ctype := mime.TypeByExtension(filepath.Ext(p))
		// Need to make a local copy of the var or else all files will
		// return the content of a single file (quirk with range).
		b := staticfiles[p]
		i.router.GET(fmt.Sprintf("/%s", p), func(c *gin.Context) {
			c.Data(http.StatusOK, ctype, b)
		})
	}

	// Setup all views
	i.router.GET("/", i.index)
	i.router.GET("/favicon.ico", i.favicon)
	i.router.GET("/robots.txt", i.robots)
	i.router.GET("/bans", i.bans)
	i.router.GET("/account_items", i.account_items)
	i.router.GET("/rounds", i.rounds)
	i.router.GET("/round/:round_id", i.round_detail)
	i.router.GET("/characters", i.characters)
	i.router.GET("/character/:char_id", i.character_detail)
	i.router.GET("/game_modes", i.game_modes)
	i.router.GET("/countries", i.countries)

	return nil
}

func (i *Instance) Serve(addr string) error {
	i.addr = addr
	defer i.cache.close()
	go i.cache.updater()
	i.logMsg("Now listening on %s", addr)
	return i.router.Run(i.addr)
}

func (i *Instance) index(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"pagetitle":   "Index",
		"Round":       i.cache.LatestRound,
		"Stats":       i.cache.GameStats,
		"LastUpdated": i.cache.LastUpdated,
		"UpdateTime":  i.cache.UpdateTime,
	})
}

func (i *Instance) favicon(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/static/favicon.ico")
}

func (i *Instance) robots(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/static/robots.txt")
}

func (i *Instance) bans(c *gin.Context) {
	ckey := c.Query("ckey")
	bans := i.DB.SearchBans(ckey)

	c.HTML(http.StatusOK, "bans.html", gin.H{
		"pagetitle": "Bans",
		"Bans":      bans,
	})
}

func (i *Instance) account_items(c *gin.Context) {
	c.HTML(http.StatusOK, "account_items.html", gin.H{
		"pagetitle":    "Account Items",
		"AccountItems": i.DB.AllAccountItems(),
	})
}

func (i *Instance) rounds(c *gin.Context) {
	c.HTML(http.StatusOK, "rounds.html", gin.H{
		"pagetitle": "Rounds",
		"Rounds":    i.DB.AllRounds(),
	})
}

func (i *Instance) round_detail(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("round_id"), 10, 0)
	if e != nil {
		id = -1
	}
	round := i.DB.GetRound(id)

	c.HTML(http.StatusOK, "round_detail.html", gin.H{
		"pagetitle": fmt.Sprintf("Round #%d", round.ID),
		"Round":     round,
		"Antags":    i.DB.GetAntags(id),
		"AILaws":    i.DB.GetAILaws(id),
		"Deaths":    i.DB.GetDeaths(id),
	})
}

func (i *Instance) characters(c *gin.Context) {
	name := c.Query("name")
	species := c.Query("species")
	chars := i.DB.SearchCharacter(species, name)

	c.HTML(http.StatusOK, "characters.html", gin.H{
		"pagetitle": "Characters",
		"Chars":     chars,
	})
}

func (i *Instance) character_detail(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("char_id"), 10, 0)
	if e != nil {
		id = -1
	}
	char := i.DB.GetCharacter(id)

	c.HTML(http.StatusOK, "character_detail.html", gin.H{
		"pagetitle": char.NiceName(),
		"Char":      char,
	})
}

func (i *Instance) game_modes(c *gin.Context) {
	c.HTML(http.StatusOK, "game_modes.html", gin.H{
		"pagetitle": "Game modes",
		"GameModes": i.cache.GameModes,
	})
}

func (i *Instance) countries(c *gin.Context) {
	c.HTML(http.StatusOK, "countries.html", gin.H{
		"pagetitle": "Countries",
		"Countries": i.cache.Countries,
	})
}
