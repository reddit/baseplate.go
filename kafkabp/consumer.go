package kafkabp

import (
	"fmt"
	"log"
	"os"

	"github.com/Shopify/sarama"
)

func main() {
	fmt.Println("vim-go")

	sarama.Logger = log.New(os.Stderr, "[sarama] ", log.LstdFlags)
}
