package generator

import (
	"fmt"
	"math/rand"
	"time"
)

type ItemGenerator struct {
	random *rand.Rand
}

func NewItemGenerator() *ItemGenerator {
	return &ItemGenerator{
		random: rand.New(rand.NewSource(time.Now().UTC().UnixNano())),
	}
}

func (g *ItemGenerator) GenerateName() string {
	adjectives := []string{
		"Vintage", "Modern", "Sleek", "Elegant", "Rustic",
		"Classic", "Minimalist", "Luxurious", "Handcrafted", "Artisanal",
		"Eco-friendly", "Sustainable", "Organic", "Premium", "Exclusive",
		"Limited Edition", "Signature", "Designer", "Custom", "Bespoke",
	}

	nouns := []string{
		"Lamp", "Chair", "Table", "Sofa", "Desk",
		"Bookshelf", "Cabinet", "Rug", "Mirror", "Clock",
		"Vase", "Sculpture", "Painting", "Print", "Photograph",
		"Cushion", "Throw", "Candle", "Plant Pot", "Ornament",
	}

	adjective := adjectives[g.random.Intn(len(adjectives))]
	noun := nouns[g.random.Intn(len(nouns))]

	return fmt.Sprintf("%s %s", adjective, noun)
}

func (g *ItemGenerator) GenerateImageURL() string {
	width := 300 + g.random.Intn(200)
	height := 300 + g.random.Intn(200)
	return fmt.Sprintf("https://picsum.photos/%d/%d", width, height)
}

func (g *ItemGenerator) GenerateItemID() string {
	return fmt.Sprintf("item_%d_%d", time.Now().UTC().UnixNano(), rand.Intn(10000))
}
