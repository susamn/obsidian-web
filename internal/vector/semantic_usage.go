package vector

import (
	"context"
	"fmt"
	"log"
	"os"
)

func main() {
	// ============================================================
	// Example: Semantic Search on Text/Markdown Files
	// ============================================================
	//
	// This shows how to:
	// 1. Train on thousands of text/markdown files
	// 2. Ask natural language questions
	// 3. Check if content exists on a topic

	ctx := context.Background()

	// ============================================================
	// STEP 1: Choose your embedding provider
	// ============================================================

	// Option A: OpenAI (recommended for quality)
	// embedder := semantic.NewOpenAIEmbedder(os.Getenv("OPENAI_API_KEY"), "text-embedding-3-small")

	// Option B: Voyage AI (great for code and technical docs)
	// embedder := semantic.NewVoyageAIEmbedder(os.Getenv("VOYAGE_API_KEY"), "voyage-3")

	// Option C: Local Ollama (free, runs locally)
	// First run: ollama pull nomic-embed-text
	// embedder := semantic.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text", 768)

	// Option D: Simple hash embedder (for testing only - not semantic!)
	embedder := NewSimpleHashEmbedder(256)

	// ============================================================
	// STEP 2: Create the search engine
	// ============================================================

	config := DefaultConfig("./my_knowledge_base", embedder)
	// Optional: customize chunking
	// config.ChunkSize = 1500    // Characters per chunk
	// config.ChunkOverlap = 300  // Overlap between chunks

	engine, err := New(config)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	fmt.Println("✓ Search engine initialized")

	// ============================================================
	// STEP 3: Train on your data
	// ============================================================

	// Option A: Train on a single file
	err = engine.TrainFile(ctx, "./docs/example.md")
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: %v", err)
	}

	// Option B: Train on an entire directory
	// This will process all .txt and .md files recursively
	count, err := engine.TrainDirectory(ctx, "./docs", []string{".txt", ".md", ".markdown"})
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: %v", err)
	}
	fmt.Printf("✓ Trained on %d files\n", count)

	// Option C: Train on custom documents
	doc := &Document{
		ID: "custom-doc-1",
		Content: `
			Machine Learning Basics

			Machine learning is a subset of artificial intelligence that enables
			systems to learn and improve from experience without being explicitly
			programmed. It focuses on developing algorithms that can access data
			and use it to learn for themselves.

			Types of Machine Learning:
			1. Supervised Learning - Uses labeled data
			2. Unsupervised Learning - Finds patterns in unlabeled data
			3. Reinforcement Learning - Learns through trial and error

			Common Applications:
			- Image recognition
			- Natural language processing
			- Recommendation systems
			- Fraud detection
		`,
		Metadata: map[string]string{
			"title":    "Machine Learning Basics",
			"category": "AI",
			"author":   "John Doe",
		},
	}
	engine.Train(ctx, doc)
	fmt.Println("✓ Trained on custom document")

	// Add more documents for demo
	trainDemoDocuments(ctx, engine)

	stats := engine.Stats()
	fmt.Printf("✓ Index stats: %d documents, %d chunks\n\n", stats["documents"], stats["chunks"])

	// ============================================================
	// STEP 4: Ask questions!
	// ============================================================

	fmt.Println("=== Asking Questions ===\n")

	// Question 1: Basic search
	question := "What is machine learning?"
	answer, results, err := engine.Ask(ctx, question)
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("Q: %s\n", question)
		fmt.Printf("A: Found %d relevant results\n", len(results))
		fmt.Println(answer)
	}

	// Question 2: Check if content exists
	fmt.Println("=== Checking Content Existence ===\n")

	topic := "neural networks"
	hasContent, relevant, err := engine.HasContent(ctx, topic)
	if err != nil {
		log.Printf("Check failed: %v", err)
	} else if hasContent {
		fmt.Printf("✓ YES, I have content about '%s'\n", topic)
		fmt.Printf("  Found %d relevant chunks\n", len(relevant))
		for i, r := range relevant {
			fmt.Printf("  %d. Score: %.4f from %s\n", i+1, r.Score, r.Metadata["title"])
		}
	} else {
		fmt.Printf("✗ NO content found about '%s'\n", topic)
	}
	fmt.Println()

	// Check another topic
	topic = "quantum computing"
	hasContent, _, err = engine.HasContent(ctx, topic)
	if err != nil {
		log.Printf("Check failed: %v", err)
	} else if hasContent {
		fmt.Printf("✓ YES, I have content about '%s'\n", topic)
	} else {
		fmt.Printf("✗ NO content found about '%s'\n", topic)
	}
	fmt.Println()

	// Question 3: Direct search with custom k
	fmt.Println("=== Raw Search Results ===\n")

	results, err = engine.Search(ctx, "types of learning algorithms", 3)
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		for i, r := range results {
			fmt.Printf("Result %d:\n", i+1)
			fmt.Printf("  Score: %.4f\n", r.Score)
			fmt.Printf("  Source: %s\n", r.Metadata["title"])
			fmt.Printf("  Content preview: %.100s...\n\n", r.Content)
		}
	}

	fmt.Println("✓ Demo completed!")
}

func trainDemoDocuments(ctx context.Context, engine *Engine) {
	docs := []*Document{
		{
			ID: "doc-neural-networks",
			Content: `
				Neural Networks and Deep Learning

				Neural networks are computing systems inspired by biological neural networks.
				They consist of interconnected nodes (neurons) organized in layers.

				Key Concepts:
				- Perceptrons: The simplest neural network unit
				- Activation Functions: ReLU, Sigmoid, Tanh
				- Backpropagation: Algorithm for training neural networks
				- Gradient Descent: Optimization method

				Deep Learning uses neural networks with many layers to learn
				complex patterns. Popular architectures include:
				- CNNs (Convolutional Neural Networks) for images
				- RNNs (Recurrent Neural Networks) for sequences
				- Transformers for natural language
			`,
			Metadata: map[string]string{"title": "Neural Networks Guide", "category": "AI"},
		},
		{
			ID: "doc-python-basics",
			Content: `
				Python Programming Basics

				Python is a high-level, interpreted programming language known
				for its simplicity and readability.

				Key Features:
				- Dynamic typing
				- Automatic memory management
				- Rich standard library
				- Cross-platform compatibility

				Common Data Structures:
				- Lists: Ordered, mutable sequences
				- Dictionaries: Key-value mappings
				- Sets: Unordered unique elements
				- Tuples: Immutable sequences

				Python is widely used in data science, web development,
				automation, and machine learning.
			`,
			Metadata: map[string]string{"title": "Python Basics", "category": "Programming"},
		},
		{
			ID: "doc-go-concurrency",
			Content: `
				Go Concurrency Patterns

				Go provides excellent support for concurrent programming through
				goroutines and channels.

				Goroutines:
				- Lightweight threads managed by Go runtime
				- Started with 'go' keyword
				- Very cheap to create (2KB stack)

				Channels:
				- Communication mechanism between goroutines
				- Can be buffered or unbuffered
				- Supports select for multiplexing

				Common Patterns:
				- Worker pools
				- Fan-out, fan-in
				- Pipeline
				- Context for cancellation
			`,
			Metadata: map[string]string{"title": "Go Concurrency", "category": "Programming"},
		},
	}

	for _, doc := range docs {
		engine.Train(ctx, doc)
	}
}
