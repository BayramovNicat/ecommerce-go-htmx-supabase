import { createClient } from "@supabase/supabase-js";

const cfg = document.getElementById("config") as HTMLElement;
const { supabaseUrl, supabaseAnonKey, successRedirect, failureRedirect } = cfg.dataset as {
	supabaseUrl: string;
	supabaseAnonKey: string;
	successRedirect: string;
	failureRedirect: string;
};

const supabase = createClient(supabaseUrl, supabaseAnonKey, {
	auth: { detectSessionInUrl: true },
});

supabase.auth.onAuthStateChange(async (event, session) => {
	if (event === "SIGNED_IN" && session?.access_token) {
		document.cookie = `sb-access-token=${session.access_token}; Path=/; Max-Age=3600; SameSite=Lax`;
		window.location.href = successRedirect;
	} else if (event === "SIGNED_OUT" || !session) {
		window.location.href = failureRedirect;
	}
});
