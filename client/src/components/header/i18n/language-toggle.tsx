import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "@/components/ui/dropdown-menu";
import { TranslateIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";

const languages = [
  { code: "en", label: "English", flag: "🇺🇸" },
  { code: "es", label: "Español", flag: "🇪🇸" }
];

export function LanguageToggle() {
  const { i18n } = useTranslation();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="cursor-pointer">
          <TranslateIcon className="h-[1.2rem] w-[1.2rem] transition-transform hover:scale-110" />
          <span className="sr-only">Change language</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        {languages.map((lang) => {
          const isActive = i18n.language.startsWith(lang.code);
          return (
            <DropdownMenuItem
              key={lang.code}
              onClick={() => i18n.changeLanguage(lang.code)}
              className="cursor-pointer flex items-center gap-2"
            >
              <span className="text-lg">{lang.flag}</span>
              <span className={isActive ? "font-medium text-primary" : ""}>
                {lang.label}
              </span>
              {isActive && <span className="ml-auto text-primary">✓</span>}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
