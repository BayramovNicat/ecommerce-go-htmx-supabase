// Bundle HTMX and Alpine.js
import htmx from 'htmx.org'
import Alpine from 'alpinejs'

// Make them globally available
window.htmx = htmx
window.Alpine = Alpine

// Cart utilities
const CART_KEY = 'cart';

function getStoredCart() {
    try {
        return JSON.parse(localStorage.getItem(CART_KEY)) || [];
    } catch (e) {
        return [];
    }
}

function saveCart(items) {
    localStorage.setItem(CART_KEY, JSON.stringify(items));
}

// Alpine component for product cart
window.productCart = function(product) {
    return {
        quantity: 1,
        addToCart() {
            const cart = getStoredCart();
            const existing = cart.find(item => item.slug === product.slug);
            if (existing) {
                existing.quantity += this.quantity;
            } else {
                cart.push({
                    slug: product.slug,
                    name: product.name,
                    price: product.price,
                    image: product.image,
                    url: product.url,
                    quantity: this.quantity
                });
            }
            saveCart(cart);
            window.location.href = '/cart';
        }
    }
}

// Start Alpine - defer to avoid blocking
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => Alpine.start());
} else {
    Alpine.start();
}
