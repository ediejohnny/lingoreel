import { t } from "../../i18n";
import type { MarkSettings, NoticeSettings, SelectionSettings } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { RestoreButton } from "./RestoreButton";
import { change, type SectionProps } from "./section";

/**
 * MarksCards edit the marks drawn over the video besides the subtitles: the
 * short notices and the frame of the line being corrected. onShow asks the
 * preview to show the mark being edited.
 */
export function MarksCards({
  view,
  draft,
  setDraft,
  onShow,
}: Pick<SectionProps, "view" | "draft" | "setDraft"> & { onShow: (mark: "notice" | "selection") => void }) {
  const options = view.options;
  const limits = options.limits;
  const defaults = options.defaults.marks;
  const marks = draft.marks;
  const setMarks = (next: MarkSettings, shown: "notice" | "selection") => {
    change(setDraft, "marks")(next);
    onShow(shown);
  };
  const notice = marks.notice;
  const setNotice =
    <K extends keyof NoticeSettings>(key: K) =>
    (value: NoticeSettings[K]) =>
      setMarks({ ...marks, notice: { ...notice, [key]: value } }, "notice");
  const selection = marks.selection;
  const setSelection =
    <K extends keyof SelectionSettings>(key: K) =>
    (value: SelectionSettings[K]) =>
      setMarks({ ...marks, selection: { ...selection, [key]: value } }, "selection");
  const fonts = options.fonts.includes(notice.fontFamily) ? options.fonts : [notice.fontFamily, ...options.fonts];

  return (
    <>
      <Card
        title={t("ui.settings.subtitles.notice.title")}
        aside={
          <div className="flex items-center gap-1">
            <RestoreButton
              onRestore={() => setMarks({ ...marks, notice: defaults.notice, noticeDuration: defaults.noticeDuration }, "notice")}
            />
            <HelpTip topic={t("ui.settings.subtitles.notice.title")}>{t("ui.settings.subtitles.notice.help")}</HelpTip>
          </div>
        }
      >
        <div className="grid grid-cols-[minmax(0,1fr)] gap-4 @lg:grid-cols-2">
          <Select
            label={t("ui.settings.style.font")}
            value={notice.fontFamily}
            options={fonts.map((font) => ({ value: font, label: font }))}
            onChange={setNotice("fontFamily")}
          />
          <NumberField
            label={t("ui.settings.style.size")}
            value={notice.fontSize}
            min={limits.fontSizeMin}
            max={limits.noticeSizeMax}
            step={1}
            unit="px"
            onChange={setNotice("fontSize")}
          />
          <ColorField label={t("ui.settings.subtitles.notice.color")} value={notice.color} onChange={setNotice("color")} />
          <ColorField
            label={t("ui.settings.subtitles.notice.background")}
            value={notice.background}
            alpha
            onChange={setNotice("background")}
          />
          <Segmented
            label={t("ui.settings.subtitles.notice.side")}
            value={notice.side}
            options={[
              { value: "left", label: t("ui.settings.style.position.left") },
              { value: "center", label: t("ui.settings.style.position.center") },
              { value: "right", label: t("ui.settings.style.position.right") },
            ]}
            onChange={setNotice("side")}
          />
          <NumberField
            label={t("ui.settings.subtitles.notice.duration")}
            value={marks.noticeDuration}
            min={limits.noticeDurationMin}
            max={limits.noticeDurationMax}
            step={0.5}
            unit="s"
            onChange={(noticeDuration) => setMarks({ ...marks, noticeDuration }, "notice")}
          />
          <NumberField
            label={t("ui.settings.subtitles.notice.margin")}
            value={notice.margin}
            min={0}
            max={limits.noticeMarginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={setNotice("margin")}
          />
          <NumberField
            label={t("ui.settings.subtitles.notice.maxWidth")}
            value={notice.maxWidth}
            min={limits.noticeWidthMin}
            max={1}
            step={0.05}
            scale={100}
            unit="%"
            onChange={setNotice("maxWidth")}
          />
        </div>
      </Card>
      <Card
        title={t("ui.settings.subtitles.selection.title")}
        aside={
          <div className="flex items-center gap-1">
            <RestoreButton onRestore={() => setMarks({ ...marks, selection: defaults.selection }, "selection")} />
            <HelpTip topic={t("ui.settings.subtitles.selection.title")}>{t("ui.settings.subtitles.selection.help")}</HelpTip>
          </div>
        }
      >
        <div className="grid grid-cols-[minmax(0,1fr)] gap-4 @lg:grid-cols-2">
          <ColorField label={t("ui.settings.subtitles.selection.color")} value={selection.color} alpha onChange={setSelection("color")} />
          <ColorField label={t("ui.settings.subtitles.selection.edge")} value={selection.edge} alpha onChange={setSelection("edge")} />
          <NumberField
            label={t("ui.settings.subtitles.selection.width")}
            value={selection.width}
            min={limits.selectionWidthMin}
            max={limits.selectionWidthMax}
            step={0.5}
            unit="px"
            onChange={setSelection("width")}
          />
          <NumberField
            label={t("ui.settings.subtitles.selection.gap")}
            value={selection.gap}
            min={0}
            max={limits.selectionGapMax}
            step={1}
            unit="px"
            onChange={setSelection("gap")}
          />
          <NumberField
            label={t("ui.settings.subtitles.selection.radius")}
            value={selection.radius}
            min={0}
            max={limits.selectionRadiusMax}
            step={0.5}
            unit="px"
            onChange={setSelection("radius")}
          />
        </div>
      </Card>
    </>
  );
}
