import { Annotation, type Extension } from "@codemirror/state";
import { EditorView, ViewPlugin, type ViewUpdate } from "@codemirror/view";
import { yRemoteSelections, yRemoteSelectionsTheme, ySyncFacet, YSyncConfig } from "y-codemirror.next";
import type * as Y from "yjs";

import { diffEnds, shouldSyncToYjs } from "../lib/imeDiff";

const fromYjsAnnotation = Annotation.define<YSyncConfig>();

function applyRemoteDelta(view: EditorView, conf: YSyncConfig, event: Y.YTextEvent) {
  const changes: { from: number; to: number; insert: string }[] = [];
  let pos = 0;
  for (const d of event.delta) {
    if (d.insert != null) {
      changes.push({ from: pos, to: pos, insert: String(d.insert) });
    } else if (d.delete != null) {
      changes.push({ from: pos, to: pos + d.delete, insert: "" });
      pos += d.delete;
    } else if (d.retain != null) {
      pos += d.retain;
    }
  }
  if (changes.length) {
    view.dispatch({ changes, annotations: [fromYjsAnnotation.of(conf)] });
  }
}

function flushLocalToYtext(view: EditorView, conf: YSyncConfig) {
  const ytext = conf.ytext;
  const cmText = view.state.doc.toString();
  const current = ytext.toString();
  if (current === cmText) {
    return;
  }
  const { start, oldMiddle, newMiddle } = diffEnds(current, cmText);
  ytext.doc?.transact(() => {
    if (oldMiddle.length) {
      ytext.delete(start, oldMiddle.length);
    }
    if (newMiddle.length) {
      ytext.insert(start, newMiddle);
    }
  }, conf);
}

const ySyncIME = ViewPlugin.fromClass(
  class {
    private conf: YSyncConfig;
    private observer: (event: Y.YTextEvent, tr: Y.Transaction) => void;
    private onCompositionEnd: () => void;

    constructor(private view: EditorView) {
      this.conf = view.state.facet(ySyncFacet);
      this.observer = (event, tr) => {
        if (tr.origin === this.conf) {
          return;
        }
        if (view.composing) {
          return;
        }
        applyRemoteDelta(view, this.conf, event);
      };
      this.conf.ytext.observe(this.observer);
      this.onCompositionEnd = () => {
        flushLocalToYtext(view, this.conf);
      };
      view.contentDOM.addEventListener("compositionend", this.onCompositionEnd);
    }

    update(update: ViewUpdate) {
      const fromYjs =
        update.transactions.length > 0 && update.transactions[0].annotation(fromYjsAnnotation) === this.conf;
      if (!shouldSyncToYjs(update.view.composing, update.docChanged, fromYjs)) {
        return;
      }
      const ytext = this.conf.ytext;
      ytext.doc?.transact(() => {
        let adj = 0;
        update.changes.iterChanges((fromA, toA, _fromB, _toB, insert) => {
          const insertText = insert.sliceString(0, insert.length, "\n");
          if (fromA !== toA) {
            ytext.delete(fromA + adj, toA - fromA);
          }
          if (insertText.length > 0) {
            ytext.insert(fromA + adj, insertText);
          }
          adj += insertText.length - (toA - fromA);
        });
      }, this.conf);
    }

    destroy() {
      this.conf.ytext.unobserve(this.observer);
      this.view.contentDOM.removeEventListener("compositionend", this.onCompositionEnd);
    }
  },
);

/** y-codemirror.next's ySync writes each IME keystroke into Y.Text and echoes it back, which resets the caret. */
export function yCollabIME(ytext: Y.Text, awareness: unknown): Extension {
  const ySyncConfig = new YSyncConfig(ytext, awareness);
  const plugins: Extension[] = [ySyncFacet.of(ySyncConfig), ySyncIME];
  if (awareness) {
    plugins.push(yRemoteSelectionsTheme, yRemoteSelections);
  }
  return plugins;
}
