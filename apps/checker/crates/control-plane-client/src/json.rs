use std::collections::BTreeMap;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum Value {
    String(String),
    Integer(u64),
}

pub(crate) fn parse_flat_object(input: &[u8]) -> Result<BTreeMap<String, Value>, ()> {
    let text = std::str::from_utf8(input).map_err(|_| ())?;
    let mut parser = Parser::new(text);
    parser.skip_ws();
    parser.expect('{')?;
    parser.skip_ws();

    let mut values = BTreeMap::new();
    if parser.consume('}') {
        parser.skip_ws();
        return if parser.finished() {
            Ok(values)
        } else {
            Err(())
        };
    }

    loop {
        parser.skip_ws();
        let key = parser.string()?;
        parser.skip_ws();
        parser.expect(':')?;
        parser.skip_ws();
        let value = if parser.peek() == Some('"') {
            Value::String(parser.string()?)
        } else {
            Value::Integer(parser.unsigned_integer()?)
        };

        if values.insert(key, value).is_some() {
            return Err(());
        }

        parser.skip_ws();
        if parser.consume('}') {
            break;
        }
        parser.expect(',')?;
    }

    parser.skip_ws();
    if parser.finished() {
        Ok(values)
    } else {
        Err(())
    }
}

struct Parser<'a> {
    input: &'a str,
    offset: usize,
}

impl<'a> Parser<'a> {
    const fn new(input: &'a str) -> Self {
        Self { input, offset: 0 }
    }

    fn finished(&self) -> bool {
        self.offset == self.input.len()
    }

    fn remaining(&self) -> &'a str {
        &self.input[self.offset..]
    }

    fn peek(&self) -> Option<char> {
        self.remaining().chars().next()
    }

    fn next(&mut self) -> Option<char> {
        let character = self.peek()?;
        self.offset += character.len_utf8();
        Some(character)
    }

    fn skip_ws(&mut self) {
        while matches!(self.peek(), Some(' ' | '\n' | '\r' | '\t')) {
            self.next();
        }
    }

    fn expect(&mut self, expected: char) -> Result<(), ()> {
        if self.next() == Some(expected) {
            Ok(())
        } else {
            Err(())
        }
    }

    fn consume(&mut self, expected: char) -> bool {
        if self.peek() == Some(expected) {
            self.next();
            true
        } else {
            false
        }
    }

    fn string(&mut self) -> Result<String, ()> {
        self.expect('"')?;
        let mut output = String::new();

        loop {
            let character = self.next().ok_or(())?;
            match character {
                '"' => return Ok(output),
                '\\' => {
                    let escaped = self.next().ok_or(())?;
                    match escaped {
                        '"' => output.push('"'),
                        '\\' => output.push('\\'),
                        '/' => output.push('/'),
                        'b' => output.push('\u{0008}'),
                        'f' => output.push('\u{000c}'),
                        'n' => output.push('\n'),
                        'r' => output.push('\r'),
                        't' => output.push('\t'),
                        'u' => self.unicode_escape(&mut output)?,
                        _ => return Err(()),
                    }
                }
                value if value <= '\u{001f}' => return Err(()),
                value => output.push(value),
            }
        }
    }

    fn unicode_escape(&mut self, output: &mut String) -> Result<(), ()> {
        let first = self.hex_quad()?;
        let scalar = if (0xd800..=0xdbff).contains(&first) {
            if self.next() != Some('\\') || self.next() != Some('u') {
                return Err(());
            }
            let second = self.hex_quad()?;
            if !(0xdc00..=0xdfff).contains(&second) {
                return Err(());
            }
            0x1_0000 + ((u32::from(first) - 0xd800) << 10) + (u32::from(second) - 0xdc00)
        } else if (0xdc00..=0xdfff).contains(&first) {
            return Err(());
        } else {
            u32::from(first)
        };

        output.push(char::from_u32(scalar).ok_or(())?);
        Ok(())
    }

    fn hex_quad(&mut self) -> Result<u16, ()> {
        let mut value = 0_u16;
        for _ in 0..4 {
            let digit = self.next().ok_or(())?.to_digit(16).ok_or(())?;
            value = (value << 4) | u16::try_from(digit).map_err(|_| ())?;
        }
        Ok(value)
    }

    fn unsigned_integer(&mut self) -> Result<u64, ()> {
        let start = self.offset;
        while matches!(self.peek(), Some('0'..='9')) {
            self.next();
        }
        if self.offset == start {
            return Err(());
        }

        let raw = &self.input[start..self.offset];
        if raw.len() > 1 && raw.starts_with('0') {
            return Err(());
        }
        raw.parse::<u64>().map_err(|_| ())
    }
}
