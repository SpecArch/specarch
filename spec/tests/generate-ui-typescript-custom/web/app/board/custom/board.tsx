import type { CustomPageProps } from "@/screens";
import { say } from "@/screens/texts";

// The live board, built by hand for this project.
export default function Board({ schema, texts }: CustomPageProps) {
  return <h1>{say(texts, schema.title)}</h1>;
}
