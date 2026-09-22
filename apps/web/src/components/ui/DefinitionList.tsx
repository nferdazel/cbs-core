import React from "react";

export interface DefinitionItem {
  label: string;
  value: React.ReactNode;
  isMono?: boolean;
}

export interface DefinitionListProps {
  items: DefinitionItem[];
  columns?: 1 | 2;
}

export const DefinitionList: React.FC<DefinitionListProps> = ({
  items,
  columns = 2,
}) => (
  <dl
    className={`grid gap-x-6 gap-y-3 ${columns === 2 ? "grid-cols-2" : "grid-cols-1"}`}
  >
    {items.map((item) => (
      <div key={item.label} className="min-w-0">
        <dt className="text-meta text-ink-600">{item.label}</dt>
        <dd
          className={`mt-0.5 break-words text-body text-ink-900 ${item.isMono ? "font-mono" : ""}`}
        >
          {item.value}
        </dd>
      </div>
    ))}
  </dl>
);
