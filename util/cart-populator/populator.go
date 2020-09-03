package main

import (
	pb "cart-populator/genproto"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"math/rand"
	"os"
	"strconv"
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

func connectToRedis(c context.Context) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     REDIS_ADDR,
		Password: "", // no password set
		DB:       0,  // use default DB
		MaxRetries: 5, // like in the c# implementation
	})
	/*_, err := rdb.Ping(c).Result()
	if err != nil {
		fmt.Println(err)
	}*/
	return rdb
}

var (
	PRODUCTS = [...]string{"0PUK6V6EV0", "1YMWWN1N4O", "2ZYFJ3GM2N", "66VCHSJNUP", "6E92ZMYYFZ", "9SIQT8TOJO", "L9ECAV7KIM", "LS4PSXUNUM", "OLJCESPC7Z"}
	REDIS_ADDR string
)

const (
	START_INDEX int64 = 100000000
	QUANTITY int32 = 5
)

func main() {
	args := os.Args[1:]		// first: redis address, second: amount of carts, third: item amount per cart
	REDIS_ADDR = args [0]
	cartAmount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	itemAmount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	client := connectToRedis(context.Background())
	for i := START_INDEX; i < START_INDEX + cartAmount; i++ {
		go addCart(i, itemAmount, cartAmount, client)	// adds cart in new thread
	}
	err = client.Close()
	if err != nil {
		fmt.Print(err)
	}
}

func addCart(cart_index int64, itemAmount int64, cartAmount int64, client *redis.Client) {
	items := []*pb.CartItem{}
	for j := 0; j < int(rand.NormFloat64() * float64(itemAmount) / 3 + float64(itemAmount)); j++ {		// normal distribution of itemAmount with with stddev=itemAmount/3
		items = append(items, &pb.CartItem{
			ProductId: PRODUCTS[j % len(PRODUCTS)],
			Quantity:  QUANTITY,
		})
	}
	err := client.Set(context.Background(), strconv.Itoa(int(cart_index)), *cartItemsToString(&items), 0).Err()
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("cart %d of %d added\n", cart_index - START_INDEX + 1, cartAmount)
	}
}