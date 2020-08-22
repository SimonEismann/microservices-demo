frontendIP = "http://frontend:8080"
products = {"0PUK6V6EV0", "1YMWWN1N4O", "2ZYFJ3GM2N", "66VCHSJNUP", "6E92ZMYYFZ", "9SIQT8TOJO", "L9ECAV7KIM", "LS4PSXUNUM", "OLJCESPC7Z"}
currencies = {"USD", "EUR", "CAD", "JPY", "GBP", "TRY"}		--same as whitelisted in frontend
quantities = {1,2,3,4,5,10}
avgCartItems = 6	--the average amount of cart items to add before cart_empty/checkout

--define functions for all API calls
--HTTP POST calls format: [POST](optional authentification_payload){optional payload}url
function frontend_home()
	return frontendIP.."/"
end

function frontend_cart_view()
	return frontendIP.."/cart"
end

function frontend_cart_add(product_id, quantity)
	return "[POST]{'product_id': "..product_id..",'quantity': "..quantity.."}"..frontend_cart_view()
end

function frontend_set_currency(currency_code)
	return "[POST]{'currency_code': "..currency_code.."}"..frontendIP.."/setCurrency"
end

function frontend_product_browse(product_id)
	return frontendIP.."/product/"..product_id
end

function frontend_cart_checkout()
	return "[POST]{'email': 'someone@example.com','street_address': '1600 Amphitheatre Parkway','zip_code': '94043','city': 'Mountain View','state': 'CA','country': 'United States','credit_card_number': '4432-8015-6152-0454','credit_card_expiration_month': '1','credit_card_expiration_year': '2039','credit_card_cvv': '672'}"..frontend_cart_view().."/checkout"
end

function frontend_cart_empty()
	return "[POST]"..frontend_cart_view().."/empty"
end
--end API call definitions

function onCycle()	--define actions for each cycle (e.g., new user token, etc.)
end

function onCall(callnum)
	toDo = math.random(7 + (2 * avgCartItems))
	if toDo == 1 then
		return frontend_home()
	elseif toDo == 2 then
		return frontend_product_browse(products[math.random(#products)])
	elseif toDo == 3 then
		return frontend_cart_view()
	elseif toDo == 4 then
		return nil		--start a new cycle
	elseif toDo == 5 then
		return frontend_set_currency(currencies[math.random(#currencies)])
	elseif toDo == 6 then
		return frontend_cart_checkout()
	elseif toDo == 7 then
		return frontend_cart_empty()
	else
		return frontend_cart_add(products[math.random(#products)], quantities[math.random(#quantities)])
	end
end