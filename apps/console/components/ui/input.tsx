"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

export const Input = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    className={cn(
      "flex h-12 w-full rounded-[20px] border border-border/80 bg-white/75 px-4 text-sm text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.65)] outline-none transition focus:border-primary/40 focus:bg-white placeholder:text-muted-foreground",
      className,
    )}
    {...props}
  />
));

Input.displayName = "Input";
