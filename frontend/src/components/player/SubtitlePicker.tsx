import { FolderOpen } from "../ui/icons";
import { t } from "../../i18n";
import { Button } from "../ui/Button";
import { Select, type Option } from "../ui/Select";

interface SubtitlePickerProps {
  label: string;
  value: string;
  options: Option[];
  note?: string;
  warning?: boolean;
  disabled?: boolean;
  onChange: (value: string) => void;
  /** onAdd asks for a subtitle file from any folder for this line; adding shows it is on its way. */
  onAdd: () => void;
  adding: boolean;
}

/** SubtitlePicker is one subtitle line's list, with a way to add a file the list does not have. */
export function SubtitlePicker({ label, value, options, note, warning, disabled, onChange, onAdd, adding }: SubtitlePickerProps) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <Select label={label} value={value} options={options} note={note} warning={warning} disabled={disabled} onChange={onChange} />
      <Button variant="ghost" icon={FolderOpen} loading={adding} disabled={disabled} onClick={onAdd} className="self-start">
        {t("ui.subtitles.addFile")}
      </Button>
    </div>
  );
}
