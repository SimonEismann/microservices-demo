package main

import (
	pb "cart-populator/genproto"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"math"
	"math/rand"
	"os"
	"strconv"
	"sync"
)

// convert cart content to csv
func cartItemsToString(items *[]*pb.CartItem) *string {
	res := ""
	if items != nil {
		for i := 0; i < len(*items); i++ {
			item := (*items)[i]
			res += item.ProductId + ";" + strconv.FormatInt(int64(item.Quantity), 10) + "\n"
		}
	}
	return &res
}

func connectToRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:       REDIS_ADDR,
		Password:   "", // no password set
		DB:         0,  // use default DB
		MaxRetries: 5,  // like in the c# implementation
	})
	/*_, err := rdb.Ping(c).Result()
	if err != nil {
		fmt.Println(err)
	}*/
	return rdb
}

var (
	PRODUCTS   = [...]string{"0PUK6V6EV0", "1YMWWN1N4O", "2ZYFJ3GM2N", "66VCHSJNUP", "6E92ZMYYFZ", "9SIQT8TOJO", "L9ECAV7KIM", "LS4PSXUNUM", "OLJCESPC7Z"}
	REDIS_ADDR string
	wg         sync.WaitGroup
)

const (
	START_INDEX      int64 = 100000000
	QUANTITY         int32 = 5
	CARTS_PER_THREAD       = 100.0
)

func main() {
	args := os.Args[1:] // first: redis address, second: amount of carts, third: item amount per cart
	REDIS_ADDR = args[0]
	cartAmount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	itemAmount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	threadAmount := int(math.Ceil(float64(cartAmount) / CARTS_PER_THREAD))
	wg.Add(threadAmount)
	for i := 0; i < threadAmount; i++ {
		offset := int64(CARTS_PER_THREAD * i)
		carts := int64(CARTS_PER_THREAD)
		if i >= threadAmount-1 {
			carts = cartAmount - offset
		}
		go addCart(START_INDEX+offset, itemAmount, carts) // adds cart in new thread
	}
	wg.Wait()
}

func addCart(cart_index_start int64, itemAmount int64, cartAmount int64) {
	defer wg.Done()
	client := connectToRedis()
	for i := cart_index_start; i < cart_index_start+cartAmount; i++ {
		items := []*pb.CartItem{}
		finalItemAmount := int(math.Max(rand.NormFloat64()*float64(itemAmount)/9+float64(itemAmount), 1.0)) // normal distribution of itemAmount with with stddev=itemAmount/9
		for j := 0; j < finalItemAmount; j++ {
			items = append(items, &pb.CartItem{
				ProductId: PRODUCTS[j%len(PRODUCTS)],
				Quantity:  QUANTITY,
			})
		}
		err := client.Set(context.Background(), strconv.Itoa(int(i)), *cartItemsToString(&items), 0).Err()
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("cart %d added with %d items\n", i, finalItemAmount)
		}
	}
	err := client.Close()
	if err != nil {
		fmt.Print(err)
	}
}
