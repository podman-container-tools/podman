local List = require("pandoc.List")

function Meta(m)
  -- Use pagetitle instead of title (prevents pandoc inserting a <H1> title)
  m.pagetitle = m.title
  m.title = nil

  if m.pagetitle ~= nil and m.pagetitle.t == "MetaInlines" then
    -- Add suffix to match the Sphinx HTML documentation
    List.extend(m.pagetitle, {pandoc.Str" \u{2014} Podman documentation"})
  end

  return m
end
