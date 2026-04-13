package analytics

import (
	"context"
	"rolando/cmd/idiscord/services"
	"rolando/cmd/ihttp/auth"
	ianalytics "rolando/internal/analytics"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"strconv"

	"github.com/disgoorg/disgo/bot"
	"github.com/gin-gonic/gin"
)

type AnalyticsController struct {
	chainsService *services.ChainsService
	ds            *bot.Client
}

func NewController(chainsService *services.ChainsService, ds *bot.Client) *AnalyticsController {
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
	chainDoc, err := s.chainsService.GetChainConf(context.Background(), chainId)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if chainDoc == nil {
		c.JSON(404, gin.H{"error": "chain not found"})
		return
	}
	chain, err := s.chainsService.GetChainConf(context.Background(), chainDoc.ID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	analyzer := s.chainsService.NewMarkovAnalyzer(chain)
	rawAnalytics, err := analyzer.GetRawAnalytics(context.Background())
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, getSerializableAnalytics(&rawAnalytics, chainDoc))
}

// GET /analytics/all, requires owner authorization
func (s *AnalyticsController) GetAllChainsAnalytics(c *gin.Context) {
	errCode, err := auth.EnsureOwner(c, s.ds)
	if err != nil {
		c.JSON(errCode, gin.H{"error": err.Error()})
		return
	}
	chains, err := s.chainsService.GetAllChains(context.Background())
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	allAnalytics := make([]gin.H, 0)
	for _, chain := range chains {
		if chain == nil {
			logger.Warnf("nil chain detected in GetAllChainsAnalytics, skipping...")
			continue
		}
		analyzer := s.chainsService.NewMarkovAnalyzer(chain)
		chainDoc, err := s.chainsService.GetChainConf(context.Background(), chain.ID)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if chainDoc == nil {
			logger.Warnf("GetChainConf returned nil for chain %s, skipping", chain.ID)
			continue
		}

		rawAnalytics, err := analyzer.GetRawAnalytics(context.Background())
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
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

	chains, total, err := s.chainsService.GetChainsPage(context.Background(), pageSize, offset)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	content := make([]gin.H, 0)
	for _, chain := range chains {
		analyzer := s.chainsService.NewMarkovAnalyzer(chain)
		chainDoc, err := s.chainsService.GetChainConf(context.Background(), chain.ID)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if chainDoc == nil {
			logger.Warnf("GetChainConf returned nil for chain %s, skipping", chain.ID)
			continue
		}

		rawAnalytics, err := analyzer.GetRawAnalytics(context.Background())
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
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

// BuildAllChainsAnalyticsMap returns a map of chain ID to serializable analytics for all chains.
// Used by the bot controller for server-side sorting of guilds by chain metrics.
func BuildAllChainsAnalyticsMap(chainsService *services.ChainsService) (map[string]gin.H, error) {
	chains, err := chainsService.GetAllChains(context.Background())
	if err != nil {
		return nil, err
	}
	m := make(map[string]gin.H, len(chains))
	for _, chain := range chains {
		chainDoc, err := chainsService.GetChainConf(context.Background(), chain.ID)
		if err != nil || chainDoc == nil {
			continue
		}
		analyzer := chainsService.NewMarkovAnalyzer(chain)
		rawAnalytics, err := analyzer.GetRawAnalytics(context.Background())
		if err != nil {
			continue
		}
		m[chain.ID] = getSerializableAnalytics(&rawAnalytics, chainDoc)
	}
	return m, nil
}

func getSerializableAnalytics(rawAnalytics *ianalytics.NumericChainAnalytics, chainDoc *repositories.ChainConfig) gin.H {
	return gin.H{
		"complexity_score": rawAnalytics.ComplexityScore,
		"gifs":             rawAnalytics.Gifs,
		"images":           rawAnalytics.Images,
		"videos":           rawAnalytics.Videos,
		"reply_rate":       rawAnalytics.ReplyRate,
		"n_gram_size":      rawAnalytics.NGramSize,
		"words":            rawAnalytics.Words,
		"messages":         rawAnalytics.Messages,
		"bytes":            rawAnalytics.Size,
		"id":               chainDoc.ID,
		"name":             chainDoc.Name,
		"max_size_mb":         chainDoc.MaxSizeMb,
		"markov_max_branches": chainDoc.MarkovMaxBranches,
		"pings_enabled":    chainDoc.Pings,
		"premium":          chainDoc.Premium,
		"trained_at":       chainDoc.TrainedAt,
		"tts_language":     chainDoc.TTSLanguage,
		"vc_join_rate":     chainDoc.VcJoinRate,
		"reaction_rate":    chainDoc.ReactionRate,
	}
}
