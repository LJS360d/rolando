package analytics

import (
	"rolando/cmd/model"
	"rolando/cmd/repositories"
	"rolando/cmd/services"
	"rolando/server/auth"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
)

type AnalyticsController struct {
	chainsService *services.ChainsService
	ds            *discordgo.Session
}

func NewController(chainsService *services.ChainsService, ds *discordgo.Session) *AnalyticsController {
	return &AnalyticsController{
		chainsService: chainsService,
		ds:            ds,
	}
}

// GET /analytics/:chain, requires member authorization
func (s *AnalyticsController) GetChainAnalytics(c *gin.Context) {
	chainId := c.Param("chain")
	errCode, err := auth.EnsureGuildMember(c, s.ds, chainId)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	chainDoc, err := s.chainsService.GetChainDocument(chainId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	chain, err := s.chainsService.GetChain(chainDoc.ID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	analyzer := model.NewMarkovChainAnalyzer(chain)
	rawAnalytics := analyzer.GetRawAnalytics()

	c.JSON(200, getSerializableAnalytics(&rawAnalytics, chainDoc))
}

// GET /analytics/all, requires owner authorization
func (s *AnalyticsController) GetAllChainsAnalytics(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	chains, err := s.chainsService.GetAllChains()
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	allAnalytics := make([]gin.H, 0)
	for _, chain := range chains {
		analyzer := model.NewMarkovChainAnalyzer(chain)
		chainDoc, err := s.chainsService.GetChainDocument(chain.ID)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Get the analytics data
		rawAnalytics := analyzer.GetRawAnalytics()
		chainAnalytics := getSerializableAnalytics(&rawAnalytics, chainDoc)
		allAnalytics = append(allAnalytics, chainAnalytics)
	}
	c.JSON(200, allAnalytics)
}

// GET /analytics, requires owner authorization
func (s *AnalyticsController) GetChainsAnalyticsPaginated(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	pageSize, err := strconv.Atoi(c.Query("pageSize"))
	if err != nil || pageSize <= 0 {
		pageSize = 8 // default page size
	}
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		page = 1 // default to first page
	}

	offset := (page - 1) * pageSize

	chains, total, err := s.chainsService.GetChainsPage(pageSize, offset)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	content := make([]gin.H, 0)
	for _, chain := range chains {
		analyzer := model.NewMarkovChainAnalyzer(chain)
		chainDoc, err := s.chainsService.GetChainDocument(chain.ID)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Get the analytics data
		rawAnalytics := analyzer.GetRawAnalytics()
		chainAnalytics := getSerializableAnalytics(&rawAnalytics, chainDoc)
		content = append(content, chainAnalytics)
	}

	c.JSON(200, gin.H{
		"data": content,
		"meta": gin.H{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// ------------ Helpers ---------------

func getSerializableAnalytics(rawAnalytics *model.NumericChainAnalytics, chainDoc *repositories.Chain) gin.H {
	return gin.H{
		"complexity_score": rawAnalytics.ComplexityScore,
		"gifs":             rawAnalytics.Gifs,
		"images":           rawAnalytics.Images,
		"videos":           rawAnalytics.Videos,
		"reply_rate":       rawAnalytics.ReplyRate,
		"words":            rawAnalytics.Words,
		"messages":         rawAnalytics.Messages,
		"bytes":            rawAnalytics.Size,
		"id":               chainDoc.ID,
		"name":             chainDoc.Name,
		"max_size_mb":      chainDoc.MaxSizeMb,
		"pings_enabled":    chainDoc.Pings,
		"trained":          chainDoc.Trained,
	}
}
