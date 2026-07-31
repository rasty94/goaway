import { Combobox } from "@/components/combobox";
import { Switch } from "@/components/ui/switch";
import { SettingRow } from "./SettingsRow";
import { ElementType } from "react";

interface SettingsSectionProps {
  label: string;
  key: string;
  explanation: string;
  default: number | string | boolean;
  options?: string[];
  // Input, Switch or Combobox; the branch below picks the props for each.
  widgetType: ElementType;
}

export const DynamicSettingsSection = ({
  settings,
  getSettingValue,
  handleSettingChange
}: {
  settings: SettingsSectionProps[];
  getSettingValue: (key: string) => number | string | boolean;
  handleSettingChange: (key: string, value: number | string | boolean) => void;
}) => (
  <>
    {settings.map(
      ({ label, key, explanation, options, widgetType: Widget }) => {
        const value = getSettingValue(key);

        return (
          <SettingRow
            key={key}
            title={label}
            description={explanation}
            action={
              <Widget
                {...(Widget === Combobox
                  ? {
                      value: String(value),
                      onChange: (v: string) => handleSettingChange(key, v),
                      options,
                      className: "w-40"
                    }
                  : Widget === Switch
                    ? {
                        checked: Boolean(value),
                        onCheckedChange: (v: boolean) =>
                          handleSettingChange(key, v)
                      }
                    : {
                        value: String(value),
                        onChange: (e: React.ChangeEvent<HTMLInputElement>) =>
                          handleSettingChange(key, e.target.value),
                        placeholder: label,
                        className: "w-40"
                      })}
              />
            }
          />
        );
      }
    )}
  </>
);
