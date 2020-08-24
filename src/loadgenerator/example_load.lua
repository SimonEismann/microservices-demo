function getNewID()
	return tostring(math.random(1000000000, 9999999999))
end

function onCycle()	--define actions for each cycle (e.g., new user token, etc.)
	userId = getNewID()
	frontendIP = "http://frontend:8080"
	products = {"0PUK6V6EV0", "1YMWWN1N4O", "2ZYFJ3GM2N", "66VCHSJNUP", "6E92ZMYYFZ", "9SIQT8TOJO", "L9ECAV7KIM", "LS4PSXUNUM", "OLJCESPC7Z"}
	currencies = {"USD", "EUR", "CAD", "JPY", "GBP", "TRY"}		--same as whitelisted in frontend
	quantities = {1,2,3,4,5,10}

	--user behavior
	amountProductBrowse = 10
	amountCartAdd = 10
end

--define functions for all API calls (normally GET)
--HTTP POST calls format: [POST](optional authentification_payload){optional payload}url
function frontend_home()
	return frontendIP.."/"
end

function frontend_cart_view(user_id)
	return frontendIP.."/cart/"..user_id
end

function frontend_cart_add(user_id, product_id, quantity)
	return "[POST]{product_id="..product_id.."&quantity="..quantity.."}"..frontendIP.."/cart"
end

function frontend_set_currency(currency_code)
	return "[POST]{currency_code="..currency_code.."}"..frontendIP.."/setCurrency"
end

function frontend_product_browse(product_id)
	return frontendIP.."/product/"..product_id
end

function frontend_cart_checkout(user_id)
	return "[POST]{user_id="..user_id.."&email=someone%40example.com&street_address=1600+Amphitheatre+Parkway&zip_code=94043&city=Mountain+View&state=CA&country=United+States&credit_card_number=4432-8015-6152-0454&credit_card_expiration_month=1&credit_card_expiration_year=2021&credit_card_cvv=672}"..frontend_cart_view().."/checkout"
end

function frontend_cart_empty(user_id)
	return "[POST]{user_id="..user_id.."}"..frontendIP.."/cart/empty"
end

function frontend_logout()
	return frontendIP.."/logout"
end
--end API call definitions

function onCall(callnum)
	if (callnum == 1) then
		return frontend_home()
	elseif (callnum <= (1 + amountProductBrowse)) then
		return frontend_product_browse(products[math.random(#products)])
	elseif (callnum == (2 + amountProductBrowse)) then
		return frontend_set_currency(currencies[math.random(#currencies)])
	elseif (callnum <= (2 + amountProductBrowse + amountCartAdd)) then
		return frontend_cart_add(userId, products[math.random(#products)], quantities[math.random(#quantities)])
	elseif (callnum == (3 + amountProductBrowse + amountCartAdd)) then
		return frontend_cart_view(userId)
	elseif (callnum == (4 + amountProductBrowse + amountCartAdd)) then
		temp = math.random(3)
		if (temp ~= 1) then
			return frontend_cart_checkout(userId)
		else
			return frontend_cart_empty(userId)
		end
	elseif (callnum == (5 + amountProductBrowse + amountCartAdd)) then
		return frontend_logout()
	else
		return nil;		--start a new cycle
	end
end
