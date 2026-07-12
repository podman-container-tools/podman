function Link(el)
  el.target = string.gsub(el.target, "%.1.md", ".html")
  return el
end
