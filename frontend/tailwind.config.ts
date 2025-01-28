// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import defaultTheme from "tailwindcss/defaultTheme";
import colors from "tailwindcss/colors";
import * as colorsTheme from "./src/vars/colors";
import type { Config } from "tailwindcss";

export default {
  content: ["./public/**/*.html", "./src/**/*.vue"],
  darkMode: "class",
  theme: {
    fontFamily: {
      sans: ["Inter var", ...defaultTheme?.fontFamily?.['sans']],
      roboto: ["Roboto Mono", "monospace"],
      firasans: ["Fira Sans", "Roboto", "sans-serif"],
    },
    extend: {
      colors: {
        "talos-gray": colors.neutral,
        "light-blue": colors.sky,
        ...colorsTheme,
      },
      animation: {
        blink: 'blink-frames 3s cubic-bezier(.6,.4,.4,.5) infinite',
        fadein: 'fadein 300ms linear',
      },
      keyframes: {
        fadein: {
          '0%': { opacity: '0' },
          '100%': { opacity: '100' },
        }
      }
    },
    screens: {
      sm: "640px",
      // => @media (min-width: 640px) { ... }

      md: "768px",
      // => @media (min-width: 768px) { ... }

      lg: "1024px",
      // => @media (min-width: 1024px) { ... }

      xl: "1280px",
      // => @media (min-width: 1280px) { ... }

      "2xl": "1536px",
      // => @media (min-width: 1536px) { ... }
    },
  },
  variants: {
    extend: {
      backgroundColor: ["active", "disabled"],
      textColor: ["active", "disabled"],
      borderColor: ["active", "disabled"],
      cursor: ["disabled"],
      textDecoration: ["active"],
    },
  },
  plugins: [],
} satisfies Config;
