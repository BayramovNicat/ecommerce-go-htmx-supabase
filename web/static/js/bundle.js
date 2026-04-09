// Bundle HTMX and Alpine.js
import htmx from "htmx.org"
import Alpine from "alpinejs"

// Make them globally available
window.htmx = htmx
window.Alpine = Alpine

// Alpine component for product cart
window.productCart = function (product) {
	return {
		quantity: 1,
		addToCart() {
			fetch("/api/cart/items", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ slug: product.slug, quantity: this.quantity }),
			}).then(() => {
				window.location.href = "/cart"
			})
		},
	}
}

// Start Alpine - defer to avoid blocking
if (document.readyState === "loading") {
	document.addEventListener("DOMContentLoaded", () => Alpine.start())
} else {
	Alpine.start()
}
