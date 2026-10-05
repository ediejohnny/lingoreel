import { Fragment, type ReactNode } from "react";

interface RichProps {
  /** text is a catalog text with its {names} left unfilled. */
  text: string;
  nodes: Record<string, ReactNode>;
}

/** Rich shows a text with nodes in place of its {names}, such as a key in <kbd>. */
export function Rich({ text, nodes }: RichProps) {
  return (
    <>
      {text.split(/(\{[a-zA-Z]+\})/).map((part, index) => {
        const name = /^\{([a-zA-Z]+)\}$/.exec(part)?.[1];
        return <Fragment key={index}>{name !== undefined && name in nodes ? nodes[name] : part}</Fragment>;
      })}
    </>
  );
}
