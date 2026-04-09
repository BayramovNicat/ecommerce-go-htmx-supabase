import { createClient } from "@supabase/supabase-js";

const btn = document.getElementById("google-login") as HTMLButtonElement | null;
if (btn) {
	const { supabaseUrl, supabaseAnonKey, oauthRedirectUrl } = btn.dataset as {
		supabaseUrl: string;
		supabaseAnonKey: string;
		oauthRedirectUrl: string;
	};

	const supabase = createClient(supabaseUrl, supabaseAnonKey, {
		auth: { detectSessionInUrl: false },
	});

	btn.addEventListener("click", async () => {
		const { error } = await supabase.auth.signInWithOAuth({
			provider: "google",
			options: {
				scopes: "email profile",
				redirectTo: oauthRedirectUrl,
			},
		});
		if (error) alert("Authentication failed: " + error.message);
	});
}
