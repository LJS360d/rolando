package services

import (
	"errors"
	"fmt"
	"rolando/internal/logger"
	"rolando/internal/model"
	"rolando/internal/repositories"
	"rolando/internal/utils"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type ChainsService struct {
	mu           sync.RWMutex
	session      *discordgo.Session
	chainsMap    map[string]*model.MarkovChain
	chainsRepo   repositories.ChainsRepository
	messagesRepo repositories.MessagesRepository
}

// NewChainsService initializes a new ChainsService.
func NewChainsService(ds *discordgo.Session, chainsRepo repositories.ChainsRepository, messagesRepo repositories.MessagesRepository) *ChainsService {
	return &ChainsService{
		chainsMap:    make(map[string]*model.MarkovChain),
		chainsRepo:   chainsRepo,
		messagesRepo: messagesRepo,
		session:      ds,
	}
}

// GetChain retrieves a Markov chain by ID, creating it if not already loaded.
func (cs *ChainsService) GetChain(id string) (*model.MarkovChain, error) {
	cs.mu.RLock()
	chain, exists := cs.chainsMap[id]
	cs.mu.RUnlock()

	if exists {
		return chain, nil
	}
	guild, err := cs.session.State.Guild(id)
	if err != nil {
		return nil, err
	}
	// Create a new chain if it doesn't exist
	return cs.CreateChain(id, guild.Name)
}

func (cs *ChainsService) GetAllChains() ([]*model.MarkovChain, error) {
	chainDocs, err := cs.chainsRepo.GetAll()
	if err != nil {
		return nil, err
	}
	var chains []*model.MarkovChain
	for _, chainDoc := range chainDocs {
		chain, _ := cs.GetChain(chainDoc.ID)
		chains = append(chains, chain)
	}
	return chains, nil
}

func (cs *ChainsService) GetChainsPage(limit, offset int) ([]*model.MarkovChain, int64, error) {
	chainDocs, total, err := cs.chainsRepo.GetChainsPage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var chains []*model.MarkovChain
	for _, chainDoc := range chainDocs {
		chain, _ := cs.GetChain(chainDoc.ID)
		chains = append(chains, chain)
	}
	return chains, total, nil
}

// GetChainDocument retrieves the chain document from the repository.
func (cs *ChainsService) GetChainDocument(id string) (*repositories.Chain, error) {
	return cs.chainsRepo.GetChainByID(id)
}

// CreateChain initializes a new Markov chain and saves it in the repository.
func (cs *ChainsService) CreateChain(id, name string) (*model.MarkovChain, error) {
	logger.Infof("Creating chain: %s", name)
	cs.mu.Lock()
	chain := model.NewMarkovChain(id, 10, 2, true, []string{}, cs.messagesRepo)
	_, exists := cs.chainsMap[id]
	if exists {
		cs.mu.Unlock()
		return nil, fmt.Errorf("chain %s already exists", name)
	}
	cs.chainsMap[id] = chain
	_, err := cs.chainsRepo.CreateChain(id, name)
	cs.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return chain, nil
}

// UpdateChainState updates the Markov chain's state with new text data.
func (cs *ChainsService) UpdateChainState(id string, text []string) (*model.MarkovChain, error) {
	chain, err := cs.GetChain(id)
	if err != nil {
		return nil, err
	}

	chain.ProvideData(text)
	return chain, nil
}

// DeleteTextData deletes specific text data from a chain.
func (cs *ChainsService) DeleteTextData(id, data string) error {
	return cs.messagesRepo.DeleteGuildMessage(id, data)
}

// UpdateChainMeta updates the chain's properties in memory and in the repository.
func (cs *ChainsService) UpdateChainMeta(id string, fields map[string]any) (*repositories.Chain, error) {
	if _, ok := fields["id"]; ok {
		return nil, errors.New("cannot change field 'id'")
	}
	chain, err := cs.GetChain(id)
	if err != nil {
		return nil, err
	}
	// Reply Rate immediate update
	if replyRateRaw, ok := fields["reply_rate"]; ok {
		replyRate, err := parseToInt(replyRateRaw)
		if err != nil {
			return nil, errors.New("reply_rate must be an integer, " + err.Error())
		}
		chain.ReplyRate = replyRate
	}
	// NGramSize immediate update
	if nGramSizeRaw, ok := fields["n_gram_size"]; ok {
		nGramSize, err := parseToInt(nGramSizeRaw)
		if err != nil {
			return nil, errors.New("n_gram_size must be an integer, " + err.Error())
		}
		messages, err := cs.GetChainMessages(id)
		if err != nil {
			return nil, errors.New("failed to retrieve messages for chain " + id + ": " + err.Error())
		}
		go func() {
			startTime := time.Now()
			logger.Infof("Updating n_gram_size for chain %s to %d", id, nGramSize)
			chain.ChangeNGramSize(nGramSize, messages)
			logger.Infof("Finished updating n_gram_size for chain %s to %d in %s", id, nGramSize, time.Since(startTime).String())
		}()
	}
	// Pings immediate update
	if pingsRaw, ok := fields["pings"]; ok {
		pings, ok := pingsRaw.(bool)
		if !ok {
			return nil, errors.New("pings must be a boolean")
		}
		chain.Pings = pings
	}
	return cs.chainsRepo.UpdateChain(id, fields)
}

// DeleteChain removes a chain from memory and the repository.
func (cs *ChainsService) DeleteChain(id string) error {
	logger.Warnf("Deleting chain: %s", id)
	cs.mu.Lock()
	delete(cs.chainsMap, id)
	cs.mu.Unlock()
	err := cs.chainsRepo.DeleteChain(id)
	if err != nil {
		return err
	}
	err = cs.messagesRepo.DeleteAllGuildMessages(id)
	if err != nil {
		return err
	}
	logger.Infof("Chain %s and associated messages deleted successfully", id)
	return nil
}

// LoadChains loads all chains from the repository into memory.
func (cs *ChainsService) LoadChains() error {
	logger.Debugln("Loading chains...")
	chains, err := cs.chainsRepo.GetAll()
	if err != nil {
		return err
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, chain := range chains {
		messages, err := cs.GetChainMessages(chain.ID)
		if err != nil {
			logger.Errorf("Error loading messages for chain %s: %v", chain.ID, err)
			continue
		}

		cs.chainsMap[chain.ID] = model.NewMarkovChain(
			chain.ID,
			chain.ReplyRate,
			chain.NGramSize,
			chain.Pings,
			messages,
			cs.messagesRepo,
		)
	}
	logger.Debugf("Loaded %d chains", len(cs.chainsMap))
	return nil
}

// GetChainMessages retrieves messages associated with a specific chain.
func (cs *ChainsService) GetChainMessages(id string) ([]string, error) {
	messages, err := cs.messagesRepo.GetAllGuildMessages(id)
	if err != nil {
		return nil, err
	}
	var texts []string
	for _, message := range messages {
		texts = append(texts, message.Content)
	}
	return texts, nil
}

// GetChainsMemUsage calculates the memory usage of all chains in memory.
func (cs *ChainsService) GetChainsMemUsage() int64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var totalSize int64
	for _, chain := range cs.chainsMap {
		totalSize += int64(utils.MeasureSize(chain))
	}
	return totalSize
}

// ---------- Helpers -----------

func parseToInt(v any) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("invalid type for integer parsing: %T", val)
	}
}
