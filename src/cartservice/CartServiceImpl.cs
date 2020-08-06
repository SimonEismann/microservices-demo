// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using cartservice.interfaces;
using Grpc.Core;
using Hipstershop;
using zipkin4net;
using static Hipstershop.CartService;

namespace cartservice
{
    // Cart wrapper to deal with grpc communication
    internal class CartServiceImpl : CartServiceBase
    {
        private ICartStore cartStore;
        private readonly static Empty Empty = new Empty();

        public CartServiceImpl(ICartStore cartStore)
        {
            this.cartStore = cartStore;
        }

        public async override Task<Empty> AddItem(AddItemRequest request, Grpc.Core.ServerCallContext context)
        {
			Console.WriteLine(context.RequestHeaders.ToString());	// DEBUG
			var trace = Trace.Create();
			trace.Record(Annotations.ServerRecv());
			trace.Record(Annotations.ServiceName("cartservice"));
			trace.Record(Annotations.Rpc("hipstershop.cartservice.additem"));
			//trace.Record(Annotations.Tag("grpc.path", "cartservice/AddItem"));
            await cartStore.AddItemAsync(request.UserId, request.Item.ProductId, request.Item.Quantity);
			trace.Record(Annotations.ServerSend());
            return Empty;
        }

        public async override Task<Empty> EmptyCart(EmptyCartRequest request, ServerCallContext context)
        {
			Console.WriteLine(context.RequestHeaders.ToString());	// DEBUG
			var trace = Trace.Create();
			trace.Record(Annotations.ServerRecv());
			trace.Record(Annotations.ServiceName("cartservice"));
			trace.Record(Annotations.Rpc("hipstershop.cartservice.emptycart"));
			//trace.Record(Annotations.Tag("grpc.path", "cartservice/EmptyCart"));
            await cartStore.EmptyCartAsync(request.UserId);
			trace.Record(Annotations.ServerSend());
            return Empty;
        }

        public async override Task<Hipstershop.Cart> GetCart(GetCartRequest request, ServerCallContext context)
        {
			Console.WriteLine(context.RequestHeaders.ToString());	// DEBUG
			var trace = Trace.Create();
			trace.Record(Annotations.ServerRecv());
			trace.Record(Annotations.ServiceName("cartservice"));
			trace.Record(Annotations.Rpc("hipstershop.cartservice.getcart"));
			//trace.Record(Annotations.Tag("grpc.path", "cartservice/GetCart"));
			var cart = await cartStore.GetCartAsync(request.UserId);
			trace.Record(Annotations.ServerSend());
            return cart;
        }
    }
}