import js from "@eslint/js";
import tseslint from "typescript-eslint";
import eslintPluginSvelte from "eslint-plugin-svelte";
import globals from "globals";

export default tseslint.config(
	js.configs.recommended,
	...tseslint.configs.recommended,
	...eslintPluginSvelte.configs["flat/recommended"],
	{
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node,
			},
		},
	},
	{
		files: ["**/*.svelte"],
		languageOptions: {
			parserOptions: {
				parser: tseslint.parser,
			},
		},
	},
	{
		ignores: [
			".svelte-kit/",
			"build/",
			"dist/",
			"node_modules/",
			"package/",
			".env",
			".env.*",
			"!.env.example",
			"pnpm-lock.yaml",
			"package-lock.json",
			"yarn.lock",
		],
	}
);
