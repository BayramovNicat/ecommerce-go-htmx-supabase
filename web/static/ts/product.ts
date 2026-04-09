// Product page: quantity controls + add to cart
// Data is passed via data-* attributes on #add-to-cart to avoid
// injecting Go template vars directly into JS.

const btn = document.getElementById("add-to-cart") as HTMLButtonElement | null;
if (btn) {
	const max = parseInt(btn.dataset.stock ?? "0", 10);
	const slug = btn.dataset.slug ?? "";
	let qty = 1;

	const display = document.getElementById("qty-display") as HTMLSpanElement;

	document.getElementById("qty-dec")?.addEventListener("click", () => {
		if (qty > 1) display.textContent = String(--qty);
	});

	document.getElementById("qty-inc")?.addEventListener("click", () => {
		if (qty < max) display.textContent = String(++qty);
	});

	btn.addEventListener("click", () => {
		fetch("/api/cart/items", {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ slug, quantity: qty }),
		}).then(() => {
			window.location.href = "/cart";
		});
	});
}
